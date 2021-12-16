package controlplane

import (
	"fmt"
	"io"
	"net/url"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	controlplaneClient "github.com/newrelic/nri-kubernetes/v2/src/controlplane/client"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane/grouper"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
)

// Providers is a struct holding pointers to all the clients Scraper needs to get data from.
// TODO: Extract this out of the package.
type Providers struct {
	K8s kubernetes.Interface
}

// Scraper takes care of getting metrics all control plane instances based on the configuration.
type Scraper struct {
	Providers
	logger                  log.Logger
	config                  *config.Config
	k8sVersion              *version.Info
	components              []component
	informerClosers         []chan<- struct{}
	podListerByNamespace    map[string]v1.PodNamespaceLister
	secretListerByNamespace map[string]v1.SecretNamespaceLister
	inClusterConfig         *rest.Config
	authenticator           controlplaneClient.Authenticator
}

// ScraperOpt are options that can be used to configure the Scraper
type ScraperOpt func(s *Scraper) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger log.Logger) ScraperOpt {
	return func(s *Scraper) error {
		if logger == nil {
			return fmt.Errorf("logger canont be nil")
		}

		s.logger = logger

		return nil
	}
}

// WithRestConfig returns an OptionFunc to change the restConfig from default empty config.
func WithRestConfig(restConfig *rest.Config) ScraperOpt {
	return func(s *Scraper) error {
		if restConfig == nil {
			return fmt.Errorf("restConfig canont be nil")
		}
		s.inClusterConfig = restConfig
		return nil
	}
}

// Close will signal internal informers to stop running.
func (s *Scraper) Close() {
	for _, ch := range s.informerClosers {
		close(ch)
	}
}

// NewScraper initialize its internal informers and components.
// After use, informers should be closed by calling Close().
func NewScraper(config *config.Config, providers Providers, options ...ScraperOpt) (*Scraper, error) {
	var err error
	s := &Scraper{
		config:    config,
		Providers: providers,
		// TODO: An empty implementation of the logger interface would be better
		logger:               log.New(false, io.Discard),
		podListerByNamespace: make(map[string]v1.PodNamespaceLister),
		components:           newComponents(config.ControlPlane),
		inClusterConfig:      &rest.Config{},
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

	// Building pod and secret lister and closers.
	s.buildListers()

	s.authenticator = controlplaneClient.NewAuthenticator(s.logger, s.secretListerByNamespace, s.inClusterConfig)

	return s, nil
}

// Run scraper collect the data populating the integration entities.
func (s *Scraper) Run(i *integration.Integration) error {
	var jobs []*scrape.Job

	for _, component := range s.components {
		var job *scrape.Job

		var err error

		// Static endpoint take precedence over autodisover and fails if external endpoint
		// cannot be scraped.
		if component.StaticEndpointConfig != nil {
			s.logger.Debugf("Using static endpoint for component %q", component.Name)

			job, err = s.externalEndpoint(component)
			if err != nil {
				return fmt.Errorf("configuring %q external endpoint: %w", component.Name, err)
			}
		} else {
			s.logger.Debugf("Autodiscovering pods for component %q", component.Name)

			job, err = s.autodiscover(component)
			if err != nil {
				return fmt.Errorf("autodiscovering %q endpoint: %w", component.Name, err)
			}
		}

		// If autodisover do not find any valid endpoint it will return a nil job and no error.
		if job != nil {
			jobs = append(jobs, job)
		}
	}

	for _, job := range jobs {
		s.logger.Debugf("Running job: %s", job.Name)

		result := job.Populate(i, s.config.ClusterName, s.logger, s.k8sVersion)

		if len(result.Errors) > 0 {
			s.logger.Infof("Error populating data from %s: %v", job.Name, result.Error())
		}
	}

	return nil
}

// externalEndpoint builds the client based on the StaticEndpointConfig and fails if
// the client probe cannot reach the endpoint.
func (s *Scraper) externalEndpoint(c component) (*scrape.Job, error) {
	connector := controlplaneClient.DefaultConnector(
		[]config.Endpoint{*c.StaticEndpointConfig},
		s.authenticator,
		s.logger,
	)

	client, err := controlplaneClient.New(connector, controlplaneClient.WithLogger(s.logger))
	if err != nil {
		return nil, fmt.Errorf("creating client for component %s failed: %v", c.Name, err)
	}

	u, err := url.Parse(c.StaticEndpointConfig.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing static endpoint url for component %s failed: %v", c.Name, err)
	}

	// Entity key will be concatenated with host info (agent replace 'localhost' for hostname even in fw mode)
	// example of etcd configured static (http://localhost:2381) entity key:'k8s:e2e-test:controlplane:etcd:minikube:2381'
	grouper := grouper.New(
		client,
		c.Queries,
		s.logger,
		u.Host,
	)

	return scrape.NewScrapeJob(string(c.Name), grouper, c.Specs), nil
}

// autodiscover will iterate over the Autodiscovery configs from a component and for each:
//  - Discover if any pod matches the selector.
//  - Build the client, which probes all the endpoints in the list.
// It uses the first autodiscovery config that can satisfy conditions above.
// It doesn't fail if no autodiscovery satisfy the contitions.
func (s *Scraper) autodiscover(c component) (*scrape.Job, error) {
	for _, autodiscover := range c.AutodiscoverConfigs {
		pod, err := s.discoverPod(autodiscover)
		if err != nil {
			return nil, fmt.Errorf("control plane component %q discovery failed: %v", c.Name, err)
		}

		if pod == nil {
			s.logger.Debugf("No %q pod found with labels %q", c.Name, autodiscover.Selector)
			continue
		}

		s.logger.Debugf("Found pod %q for %q with labels %q", pod.Name, c.Name, autodiscover.Selector)

		connector := controlplaneClient.DefaultConnector(
			autodiscover.Endpoints,
			s.authenticator,
			s.logger,
		)

		client, err := controlplaneClient.New(connector, controlplaneClient.WithLogger(s.logger))
		if err != nil {
			s.logger.Debugf("Failed creating %q client: %v", c.Name, err)
			continue
		}

		grouper := grouper.New(
			client,
			c.Queries,
			s.logger,
			pod.Name,
		)

		return scrape.NewScrapeJob(string(c.Name), grouper, c.Specs), nil
	}

	s.logger.Debugf("No %q pod has been discovered", c.Name)

	return nil, nil
}

func (s *Scraper) discoverPod(autodiscover config.AutodiscoverControlPlane) (*corev1.Pod, error) {
	// looks for the pod match a set of defined labels
	podLister, ok := s.podListerByNamespace[autodiscover.Namespace]
	if !ok {
		return nil, fmt.Errorf("pod lister for namespace: %s not found", autodiscover.Namespace)
	}

	labelsSet, _ := labels.ConvertSelectorToLabelsMap(autodiscover.Selector)

	selector := labels.SelectorFromSet(labelsSet)

	pods, err := podLister.List(selector)
	if err != nil {
		return nil, fmt.Errorf("fail to list pods for selector: %v", labelsSet)
	}

	s.logger.Debugf("%d pods found with labels %q", len(pods), autodiscover.Selector)
	// Validation of returned pods.
	for _, pod := range pods {
		if autodiscover.MatchNode && pod.Spec.NodeName != s.config.NodeName {
			s.logger.Debugf("Discarding pod: %s running outside the node", pod.Name)
			continue
		}
		return pod, nil
	}

	return nil, nil
}

// buildListers populates podListerByNamespace and secretListerByNamespace with a
// lister for each namespace needed.
func (s *Scraper) buildListers() {
	for _, component := range s.components {
		if component.StaticEndpointConfig != nil {
			s.addSecretLister(component.StaticEndpointConfig.Auth)
		}

		for _, autodiscover := range component.AutodiscoverConfigs {
			s.addPodLister(autodiscover.Namespace)

			for _, endpoint := range autodiscover.Endpoints {
				s.addSecretLister(endpoint.Auth)
			}
		}
	}
}

func (s *Scraper) addPodLister(namespace string) {
	if _, ok := s.podListerByNamespace[namespace]; !ok {
		s.logger.Debugf("Generating a new Pod lister for namespace %q.", namespace)

		podLister, informerCloser := discovery.NewPodNamespaceLister(discovery.PodListerConfig{
			Client:    s.K8s,
			Namespace: namespace,
		})

		s.podListerByNamespace[namespace] = podLister
		s.informerClosers = append(s.informerClosers, informerCloser)
	}
}

func (s *Scraper) addSecretLister(auth *config.Auth) {
	if auth != nil && auth.MTLS != nil {
		namespace := controlplaneClient.DefaultSecretNamespace
		if auth.MTLS.TLSSecretNamespace != "" {
			namespace = auth.MTLS.TLSSecretNamespace
		}

		s.logger.Debugf("Generating a new Secret lister for namespace %q.", namespace)

		secretLister, informerCloser := discovery.NewSecretNamespaceLister(discovery.SecretListerConfig{
			Client:    s.K8s,
			Namespace: namespace,
		})

		s.secretListerByNamespace[namespace] = secretLister
		s.informerClosers = append(s.informerClosers, informerCloser)
	}
}
