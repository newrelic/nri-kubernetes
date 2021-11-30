package controlplane

import (
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	controlplaneClient "github.com/newrelic/nri-kubernetes/v2/src/controlplane/client"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane/grouper"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
)

// Providers is a struct holding pointers to all the clients Scraper needs to get data from.
// TODO: Extract this out of the package.
type Providers struct {
	K8s kubernetes.Interface
}

// Scraper takes care of getting metrics from an autodiscovered CP instances.
type Scraper struct {
	Providers
	logger               log.Logger
	config               *config.Config
	k8sVersion           *version.Info
	components           []component
	informerClosers      []chan<- struct{}
	podListerByNamespace map[string]v1.PodLister
}

// ScraperOpt are options that can be used to configure the Scraper
type ScraperOpt func(s *Scraper) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger log.Logger) ScraperOpt {
	return func(s *Scraper) error {
		s.logger = logger
		return nil
	}
}

// Close will signal internal informers to stop running.
func (s *Scraper) Close() {
	for _, ch := range s.informerClosers {
		close(ch)
	}
}

// NewScraper builds a new Scraper, initializing its internal informers. After use, informers should be closed by calling
func NewScraper(config *config.Config, providers Providers, options ...ScraperOpt) (*Scraper, error) {
	var err error
	s := &Scraper{
		config:    config,
		Providers: providers,
		// TODO: An empty implementation of the logger interface would be better
		logger:               log.New(false, io.Discard),
		podListerByNamespace: make(map[string]v1.PodLister),
		components:           newComponents(config.ControlPlane),
	}

	for i, opt := range options {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("applying config option #%d: %w", i, err)
		}
	}

	// TODO If this could change without a restart of the pod we should run it each time we scrape data,
	// possibly with a reasonable cache Es: NewCachedDiscoveryClientForConfig
	s.k8sVersion, err = providers.K8s.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("fetching K8s version: %w", err)
	}

	s.buildLister()

	return s, nil
}

// Run scraper collect the data populating the integration entities
func (s *Scraper) Run(i *integration.Integration) error {
	for _, component := range s.components {
		// TODO condition to scrape endpoint directly

		for _, autodiscover := range component.AutodiscoverConfigs {
			podName, err := s.discoverPod(autodiscover)
			if err != nil {
				return fmt.Errorf("control plane component %s discovery failed: %v", component.Name, err)
			}

			if podName == "" {
				s.logger.Debugf("No pod found for component: %s", component.Name)
				continue
			}

			grouper := grouper.New(
				s.deprecatedClient(autodiscover, podName),
				component.Queries,
				s.logger,
				podName,
			)

			job := scrape.NewScrapeJob(string(component.Name), grouper, component.Specs)

			s.logger.Debugf("Running job: %s", job.Name)

			result := job.Populate(i, s.config.ClusterName, s.logger, s.k8sVersion)

			if len(result.Errors) > 0 {
				s.logger.Infof("Error populating data from %s: %v", job.Name, result.Error())
			}
		}
	}

	return nil
}

func (s *Scraper) deprecatedClient(c config.AutodiscoverControlPlane, podName string) client.HTTPClient {
	timeout := 500 * time.Millisecond

	authMethod := controlplaneClient.None
	// // Let mTLS take precedence over service account
	// switch {
	// case c.UseMTLSAuthentication:
	// 	authMethod = controlplaneClient.MTLS
	// case c.UseServiceAccountAuthentication:
	// 	authMethod = controlplaneClient.ServiceAccount
	// default:
	// 	authMethod = controlplaneClient.None
	// }

	// TODO client probably receives a list of urls and do the try logic
	var endpointURL *url.URL
	if len(c.URL) > 0 {
		endpointURL, _ = url.Parse(c.URL[0])
	}
	return controlplaneClient.New(
		authMethod,
		"", // c.TLSSecretName,
		"", // c.TLSSecretNamespace,
		s.logger,
		s.K8s,
		*endpointURL,
		url.URL{}, // c.SecureEndpoint,
		s.config.NodeIP,
		podName,
		false, // c.InsecureFallback,
		timeout,
	)
}

func (s *Scraper) discoverPod(autodiscover config.AutodiscoverControlPlane) (string, error) {
	// looks for the pod match a set of defined labels
	podLister, ok := s.podListerByNamespace[autodiscover.Namespace]
	if !ok {
		return "", fmt.Errorf("pod lister for namespace: %s not found", autodiscover.Namespace)
	}

	labelsSet, _ := labels.ConvertSelectorToLabelsMap(autodiscover.Selector)

	selector := labels.SelectorFromSet(labels.Set(labelsSet))
	pods, err := podLister.List(selector)
	if err != nil {
		return "", fmt.Errorf("fail to list pods for selector: %v", labelsSet)
	}

	// Validation of returned pods.
	for _, pod := range pods {
		if autodiscover.MatchNode && pod.Spec.NodeName != s.config.NodeName {
			s.logger.Debugf("discarding pod: %s running outside the node", pod.Name)
			continue
		}
		return pod.Name, nil
	}

	return "", nil
}

func (s *Scraper) buildLister() {
	for _, component := range s.components {
		for _, autodiscover := range component.AutodiscoverConfigs {
			if _, ok := s.podListerByNamespace[autodiscover.Namespace]; !ok {
				podLister, informerCloser := discovery.NewPodsLister(discovery.PodsListerConfig{
					Client:    s.K8s,
					Namespace: autodiscover.Namespace,
				})
				s.podListerByNamespace[autodiscover.Namespace] = podLister
				s.informerClosers = append(s.informerClosers, informerCloser)
			}
		}
	}
}
