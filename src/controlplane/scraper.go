package controlplane

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	controlplaneClient "github.com/newrelic/nri-kubernetes/v3/src/controlplane/client"
	"github.com/newrelic/nri-kubernetes/v3/src/controlplane/client/authenticator"
	"github.com/newrelic/nri-kubernetes/v3/src/controlplane/client/connector"
	"github.com/newrelic/nri-kubernetes/v3/src/controlplane/discoverer"
	"github.com/newrelic/nri-kubernetes/v3/src/controlplane/grouper"
	"github.com/newrelic/nri-kubernetes/v3/src/scrape"
)

// Providers is a struct holding pointers to all the clients Scraper needs to get data from.
// TODO: Extract this out of the package.
type Providers struct {
	K8s kubernetes.Interface
}

// Scraper takes care of getting metrics all control plane instances based on the configuration.
type Scraper struct {
	Providers
	logger          *log.Logger
	config          *config.Config
	k8sVersion      *version.Info
	components      []component
	informerClosers []chan<- struct{}
	podDiscoverer   discoverer.PodDiscoverer
	inClusterConfig *rest.Config
	authenticator   authenticator.Authenticator
}

// ScraperOpt are options that can be used to configure the Scraper.
type ScraperOpt func(s *Scraper) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger *log.Logger) ScraperOpt {
	return func(s *Scraper) error {
		s.logger = logger

		return nil
	}
}

// WithRestConfig returns an OptionFunc to change the restConfig from default empty config.
func WithRestConfig(restConfig *rest.Config) ScraperOpt {
	return func(s *Scraper) error {
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
	s := &Scraper{
		config:          config,
		Providers:       providers,
		logger:          logutil.Discard,
		components:      newComponents(config.ControlPlane),
		inClusterConfig: &rest.Config{},
	}

	for i, opt := range options {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("applying config option #%d: %w", i, err)
		}
	}

	var err error
	// TODO If this could change without a restart of the pod we should run it each time we scrape data,
	// possibly with a reasonable cache Es: NewCachedDiscoveryClientForConfig
	s.k8sVersion, err = providers.K8s.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("fetching K8s version: %w", err)
	}

	secretListerer, informerCloser := discovery.NewNamespaceSecretListerer(discovery.SecretListererConfig{
		Client:     s.K8s,
		Namespaces: secretNamespaces(s.components),
	})

	s.informerClosers = append(s.informerClosers, informerCloser)

	s.authenticator, err = authenticator.New(
		authenticator.Config{
			SecretListerer:  secretListerer,
			InClusterConfig: s.inClusterConfig,
		},
		authenticator.WithLogger(s.logger),
	)
	if err != nil {
		return nil, fmt.Errorf("creating authenticator: %w", err)
	}

	podListerer, informerCloser := discovery.NewNamespacePodListerer(discovery.PodListererConfig{
		Client:     s.K8s,
		Namespaces: autodiscoverNamespaces(s.components),
	})

	s.informerClosers = append(s.informerClosers, informerCloser)

	s.podDiscoverer, err = discoverer.New(
		discoverer.Config{
			PodListerer: podListerer,
			NodeName:    config.NodeName,
		},
		discoverer.WithLogger(s.logger),
	)
	if err != nil {
		return nil, fmt.Errorf("creating pod discoverer: %w", err)
	}

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
			s.logger.Debugf("Using static endpoint for %q", component.Name)

			job, err = s.externalEndpoint(component)
			if err != nil {
				return fmt.Errorf("configuring %q external endpoint: %w", component.Name, err)
			}
		} else {
			s.logger.Debugf("Autodiscovering pods for %q", component.Name)

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
			if result.Populated {
				s.logger.Tracef("Error populating data from %s: %v", job.Name, result.Error())
			} else {
				s.logger.Warnf("Error populating data from %s: %v", job.Name, result.Error())
			}
		}
	}

	return nil
}

// externalEndpoint builds the client based on the StaticEndpointConfig and fails if
// the client probe cannot reach the endpoint.
func (s *Scraper) externalEndpoint(c component) (*scrape.Job, error) {
	connector, err := connector.New(
		connector.Config{
			Authenticator: s.authenticator,
			Endpoints:     []config.Endpoint{*c.StaticEndpointConfig},
			Timeout:       s.config.ControlPlane.Timeout,
		},
		connector.WithLogger(s.logger),
	)
	if err != nil {
		return nil, fmt.Errorf("creating connector for %q failed: %w", c.Name, err)
	}

	client, err := controlplaneClient.New(
		connector,
		controlplaneClient.WithLogger(s.logger),
		controlplaneClient.WithMaxRetries(s.config.ControlPlane.Retries),
	)
	if err != nil {
		return nil, fmt.Errorf("creating client for %q failed: %w", c.Name, err)
	}

	u, err := url.Parse(c.StaticEndpointConfig.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing static endpoint url for %q failed: %w", c.Name, err)
	}

	// Entity key will be concatenated with host info (agent replace 'localhost' for hostname even in fw mode)
	// example of etcd configured static (http://localhost:2381) entity key:'k8s:e2e-test:controlplane:etcd:minikube:2381'
	grouper := grouper.New(
		client.MetricFamiliesGetFunc(),
		c.Queries,
		s.logger,
		u.Host,
	)

	return scrape.NewScrapeJob(string(c.Name), grouper, c.Specs), nil
}

// autodiscover will iterate over the Autodiscovery configs from a component and for each:
//   - Discover if any pod matches the selector.
//   - Build the client, which probes all the endpoints in the list.
//
// It uses the first autodiscovery config that can satisfy conditions above.
// It doesn't fail if no autodiscovery satisfy the conditions.
func (s *Scraper) autodiscover(c component) (*scrape.Job, error) {
	for _, autodiscover := range c.AutodiscoverConfigs {
		pod, err := s.podDiscoverer.Discover(autodiscover)
		if errors.Is(err, discoverer.ErrPodNotFound) {
			s.logger.Debugf("No pod found for %q with labels %q in namespace %q", c.Name, autodiscover.Selector, autodiscover.Namespace)
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("discovering pod for %q: %w", c.Name, err)
		}

		s.logger.Debugf("Found pod %q for %q with labels %q", pod.Name, c.Name, autodiscover.Selector)

		connector, err := connector.New(
			connector.Config{
				Authenticator: s.authenticator,
				Endpoints:     autodiscover.Endpoints,
				Timeout:       s.config.ControlPlane.Timeout,
			},
			connector.WithLogger(s.logger),
		)
		if err != nil {
			return nil, fmt.Errorf("creating connector for %q failed: %w", c.Name, err)
		}

		client, err := controlplaneClient.New(
			connector,
			controlplaneClient.WithLogger(s.logger),
			controlplaneClient.WithMaxRetries(s.config.ControlPlane.Retries),
		)
		if err != nil {
			s.logger.Debugf("Failed creating %q client: %v", c.Name, err)
			continue
		}

		grouper := grouper.New(
			client.MetricFamiliesGetFunc(),
			c.Queries,
			s.logger,
			pod.Name,
		)

		return scrape.NewScrapeJob(string(c.Name), grouper, c.Specs), nil
	}

	s.logger.Debugf("No %q pod has been discovered", c.Name)

	return nil, nil
}
