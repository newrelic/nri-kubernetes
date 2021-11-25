package controlplane

import (
	"fmt"
	"io"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	controlplaneClient "github.com/newrelic/nri-kubernetes/v2/src/controlplane/client"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane/grouper"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
	corev1 "k8s.io/api/core/v1"
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
	config               *config.Mock
	k8sVersion           *version.Info
	components           []Component
	informerClosers      []chan<- struct{}
	PodListerByNamespace map[string]v1.PodLister
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
func NewScraper(config *config.Mock, providers Providers, options ...ScraperOpt) (*Scraper, error) {
	var err error
	s := &Scraper{
		config:    config,
		Providers: providers,
		// TODO: An empty implementation of the logger interface would be better
		logger:               log.New(false, io.Discard),
		PodListerByNamespace: make(map[string]v1.PodLister),
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

	s.buildDiscoverer(s.buildComponents())

	return s, nil
}

// Run scraper collect the data populating the integration entities
func (s *Scraper) Run(i *integration.Integration) error {
	for _, component := range s.components {

		pod, err := s.discoverPod(component)
		if err != nil {
			return fmt.Errorf("control plane component %s discovery failed: %v", component.Name, err)
		}

		if pod == nil {
			s.logger.Debugf("No pod found for component: %s", component.Name)
			continue
		}

		grouper := grouper.New(
			s.deprecatedClient(component, pod.Name),
			component.Queries,
			s.logger,
			pod.Name,
		)

		job := scrape.NewScrapeJob(string(component.Name), grouper, component.Specs)

		s.logger.Debugf("Running job: %s", job.Name)

		result := job.Populate(i, s.config.ClusterName, s.logger, s.k8sVersion)

		if len(result.Errors) > 0 {
			s.logger.Infof("Error populating data from %s: %v", job.Name, result.Error())
		}
	}

	return nil
}

func (s *Scraper) deprecatedClient(c Component, podName string) client.HTTPClient {
	timeout := 500 * time.Millisecond

	authMethod := controlplaneClient.None
	// Let mTLS take precedence over service account
	switch {
	case c.UseMTLSAuthentication:
		authMethod = controlplaneClient.MTLS
	case c.UseServiceAccountAuthentication:
		authMethod = controlplaneClient.ServiceAccount
	default:
		authMethod = controlplaneClient.None
	}

	return controlplaneClient.New(
		authMethod,
		c.TLSSecretName,
		c.TLSSecretNamespace,
		s.logger,
		s.K8s,
		c.Endpoint,
		c.SecureEndpoint,
		s.config.NodeIP,
		podName,
		c.InsecureFallback,
		timeout,
	)
}

func (s *Scraper) discoverPod(c Component) (*corev1.Pod, error) {
	var discoveredPod *corev1.Pod
	// looks for the pod match a set of defined labels
	for _, l := range c.Labels {
		podLister, ok := s.PodListerByNamespace[c.Namespace]
		if !ok {
			return nil, fmt.Errorf("pod lister for namespace: %s not found for component: %s", c.Namespace, c.Name)
		}

		selector := labels.SelectorFromSet(labels.Set(l))
		pods, err := podLister.List(selector)
		if err != nil {
			return nil, fmt.Errorf("fail to list pods for component:%v selector: %v", c.Name, l)
		}

		// Validation of returned pods.
		for _, pod := range pods {
			if pod.Spec.NodeName != s.config.NodeName {
				s.logger.Debugf("discarding pod: %s running outside the node", pod.Name)
				continue
			}
			discoveredPod = pod
			break
		}

	}

	if discoveredPod == nil {
		return nil, nil
	}

	return discoveredPod, nil
}

func (s *Scraper) buildDiscoverer(components []Component) {
	for _, component := range components {
		// TODO will be taken from config for each group of labels
		// this will became into n discoverers as groups of labels the component have
		// to allow multiple namespaces and multiple labels sets.
		component.Namespace = "kube-system"

		if _, ok := s.PodListerByNamespace[component.Namespace]; !ok {

			podLister, informerCloser := discovery.NewPodsLister(discovery.PodsListerConfig{
				Client:    s.K8s,
				Namespace: component.Namespace,
			})
			s.PodListerByNamespace[component.Namespace] = podLister
			s.informerClosers = append(s.informerClosers, informerCloser)
		}

		s.components = append(s.components, component)
	}
}

func (s *Scraper) buildComponents() []Component {
	var opts []ComponentOption

	if s.config.ETCD.EtcdTLSSecretName != "" {
		opts = append(opts, WithEtcdTLSConfig(s.config.ETCD.EtcdTLSSecretName, s.config.ETCD.EtcdTLSSecretNamespace))
	}

	if s.config.ETCD.EtcdEndpointURL != "" {
		opts = append(opts, WithEndpointURL(Etcd, s.config.ETCD.EtcdEndpointURL))
	}

	if s.config.APIServer.APIServerEndpointURL != "" {
		opts = append(opts, WithEndpointURL(APIServer, s.config.APIServer.APIServerEndpointURL))
	}

	if s.config.Scheduler.SchedulerEndpointURL != "" {
		opts = append(opts, WithEndpointURL(Scheduler, s.config.Scheduler.SchedulerEndpointURL))
	}

	if s.config.ControllerManager.ControllerManagerEndpointURL != "" {
		opts = append(opts, WithEndpointURL(ControllerManager, s.config.ControllerManager.ControllerManagerEndpointURL))
	}

	return BuildComponentList(opts...)
}
