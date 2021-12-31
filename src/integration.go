package integration

import (
	"context"
	"fmt"
	"net"
	"path"
	"runtime"
	"strings"
	"time"

	sdkIntegration "github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/logutil"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm"
	ksmClient "github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet"
	kubeletClient "github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
	"github.com/newrelic/nri-kubernetes/v2/src/sink"
	"github.com/sethgrid/pester"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Integration is the main object of the package, which takes care of building the kubernetes client, as well as
// instantiating and running the different scrapers.
type Integration struct {
	metadata       Metadata
	Logger         *logrus.Logger
	config         *config.Config
	sdkIntegration *sdkIntegration.Integration
	clients        *clusterClients
	scrapers       struct {
		kubelet      *kubelet.Scraper
		ksm          *ksm.Scraper
		controlplane *controlplane.Scraper
	}
}

// TODO: After refactoring groupers et. al the only client held by the integration would be kubernetes.Interface.
type clusterClients struct {
	k8s      kubernetes.Interface
	ksm      prometheus.MetricFamiliesGetFunc
	cAdvisor prometheus.MetricFamiliesGetFunc
	kubelet  client.HTTPGetter
}

// New returns a new Integration with the provided metadata and a logutil.Discard Logger.
func New(meta Metadata) *Integration {
	return &Integration{
		metadata: meta,
		Logger:   logutil.Discard,
	}
}

// Run configures the integration and starts the collection loop, blocking until an error occurs.
func (i *Integration) Run() error {
	if err := i.setup(); err != nil {
		return fmt.Errorf("setting up integration: %w", err)
	}

	defer i.cleanup()

	if err := i.run(); err != nil {
		return fmt.Errorf("running integration: %w", err)
	}

	return nil
}

// setup loads the configuration and builds clients and scrapers.
func (i *Integration) setup() error {
	c, err := config.LoadConfig(config.FilePath, config.FileName)
	if err != nil {
		return fmt.Errorf("loading config file: %w", err)
	}

	if c.Verbose {
		i.Logger.SetLevel(logrus.DebugLevel)
	}

	sdkI, err := i.createIntegrationWithHTTPSink(c.HTTPServerPort)
	if err != nil {
		return fmt.Errorf("creating sdk integration: %w", err)
	}
	i.sdkIntegration = sdkI

	i.Logger.Debug(i.metadata)

	clients, err := i.buildClients()
	if err != nil {
		return fmt.Errorf("building clients: %w", err)
	}
	i.clients = clients

	var kubeletScraper *kubelet.Scraper
	if c.Kubelet.Enabled {
		i.Logger.Info("Configuring kubelet scraper...")

		kubeletScraper, err = i.kubeletScraper()
		if err != nil {
			return fmt.Errorf("setting up ksm scraper: %w", err)
		}
	}
	i.scrapers.kubelet = kubeletScraper

	var ksmScraper *ksm.Scraper
	if c.KSM.Enabled {
		i.Logger.Info("Configuring KSM scraper...")
		ksmScraper, err = i.ksmScraper()
		if err != nil {
			return fmt.Errorf("setting up ksm scraper: %w", err)
		}
	}
	i.scrapers.ksm = ksmScraper

	var controlplaneScraper *controlplane.Scraper
	if c.ControlPlane.Enabled {
		i.Logger.Info("Configuring ControlPlane scraper...")
		controlplaneScraper, err = i.controlPlaneScraper()
		if err != nil {
			return fmt.Errorf("setting up control plane scraper: %w", err)
		}
	}
	i.scrapers.controlplane = controlplaneScraper

	i.Logger.Info("Configuration done.")

	return nil
}

// run executes the main collection loop, blocking until an error occurs.
func (i *Integration) run() error {
	for {
		start := time.Now()

		i.Logger.Debug("Starting collection loop...")

		// TODO: Consider parallelizing this. For now it's not necessary since different instances of the integration
		// should run only one scraper.
		if i.config.KSM.Enabled {
			i.Logger.Debug("Running KSM scraper...")
			err := i.scrapers.ksm.Run(i.sdkIntegration)
			if err != nil {
				return fmt.Errorf("retrieving ksm data: %w", err)
			}
		}

		if i.config.Kubelet.Enabled {
			i.Logger.Debug("Running Kubelet scraper...")
			err := i.scrapers.kubelet.Run(i.sdkIntegration)
			if err != nil {
				return fmt.Errorf("retrieving kubelet data: %w", err)
			}
		}

		if i.config.ControlPlane.Enabled {
			i.Logger.Debug("Running Control Plane scraper...")
			err := i.scrapers.controlplane.Run(i.sdkIntegration)
			if err != nil {
				return fmt.Errorf("retrieving control plane data: %w", err)
			}
		}

		i.Logger.Debugf("Collection done.")
		i.Logger.Debugf("Pushing metrics to agent sidecar...")
		err := i.sdkIntegration.Publish()
		if err != nil {
			return fmt.Errorf("publishing metrics: %w", err)
		}
		i.Logger.Debugf("Metrics pushed.")

		sleep := i.config.Interval - time.Since(start)
		i.Logger.Debugf("Collecting again in %v.", sleep)

		// Sleep interval minus the time that took to scrape.
		time.Sleep(sleep)
	}
}

// cleanup closes any open scraper, which will in turn cancel any kubernetes informer used by them.
func (i *Integration) cleanup() {
	if i.scrapers.ksm != nil {
		i.Logger.Info("Cleaning up KSM informers")
		i.scrapers.ksm.Close()
	}

	if i.scrapers.controlplane != nil {
		i.Logger.Info("Cleaning up control plane informers")
		i.scrapers.controlplane.Close()
	}
}

// ksmScraper returns a KSM Scraper using the local configuration, clients, and logger.
func (i *Integration) ksmScraper() (*ksm.Scraper, error) {
	providers := ksm.Providers{
		K8s: i.clients.k8s,
		KSM: i.clients.ksm,
	}

	ksmScraper, err := ksm.NewScraper(i.config, providers, ksm.WithLogger(i.Logger))
	if err != nil {
		return nil, fmt.Errorf("building KSM scraper: %w", err)
	}

	return ksmScraper, nil
}

// ksmScraper returns a Control Plane Scraper using the local configuration, clients, and logger.
func (i *Integration) controlPlaneScraper() (*controlplane.Scraper, error) {
	providers := controlplane.Providers{
		K8s: i.clients.k8s,
	}

	restConfig, err := i.k8sConfig(i.config)
	if err != nil {
		return nil, err
	}

	controlplaneScraper, err := controlplane.NewScraper(
		i.config,
		providers,
		controlplane.WithLogger(i.Logger),
		controlplane.WithRestConfig(restConfig),
	)
	if err != nil {
		return nil, fmt.Errorf("building KSM scraper: %w", err)
	}

	return controlplaneScraper, nil
}

// kubeletScraper returns a Kubelet Scraper using the local configuration, clients, and logger.
func (i *Integration) kubeletScraper() (*kubelet.Scraper, error) {
	providers := kubelet.Providers{
		K8s:      i.clients.k8s,
		Kubelet:  i.clients.kubelet,
		CAdvisor: i.clients.cAdvisor,
	}
	ksmScraper, err := kubelet.NewScraper(i.config, providers, kubelet.WithLogger(i.Logger))
	if err != nil {
		return nil, fmt.Errorf("building kubelet scraper: %w", err)
	}

	return ksmScraper, nil
}

// buildClients creates Kubernetes, KSM and Kubelet clients.
// TODO: Kubelet/KSM clients should be created by the scrapers.
func (i *Integration) buildClients() (*clusterClients, error) {
	k8sConfig, err := i.k8sConfig(i.config)
	if err != nil {
		return nil, fmt.Errorf("retrieving k8s config: %w", err)
	}

	k8s, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("building kubernetes client: %w", err)
	}

	var ksmCli *ksmClient.Client
	if i.config.KSM.Enabled {
		ksmCli, err = ksmClient.New(ksmClient.WithLogger(i.Logger))
		if err != nil {
			return nil, fmt.Errorf("building KSM client: %w", err)
		}
	}

	var kubeletCli *kubeletClient.Client
	if i.config.Kubelet.Enabled {
		kubeletCli, err = kubeletClient.New(kubeletClient.DefaultConnector(k8s, i.config, k8sConfig, i.Logger), kubeletClient.WithLogger(i.Logger))
		if err != nil {
			return nil, fmt.Errorf("building Kubelet client: %w", err)
		}
	}

	return &clusterClients{
		k8s:      k8s,
		ksm:      ksmCli,
		kubelet:  kubeletCli,
		cAdvisor: kubeletCli,
	}, nil
}

// createIntegrationWithHTTPSink returns an SDK integratio configured to write to the HTTP port of an agent sidecar.
func (i *Integration) createIntegrationWithHTTPSink(httpServerPort string) (*sdkIntegration.Integration, error) {
	c := pester.New()
	c.Backoff = pester.LinearBackoff
	c.MaxRetries = 5
	c.Timeout = sink.DefaultRequestTimeout
	c.LogHook = func(e pester.ErrEntry) {
		i.Logger.Debugf("sending data to httpSink: %q", e)
	}

	endpoint := net.JoinHostPort(sink.DefaultAgentForwarderhost, httpServerPort)

	sinkOptions := sink.HTTPSinkOptions{
		URL:        fmt.Sprintf("http://%s%s", endpoint, sink.DefaultAgentForwarderPath),
		Client:     c,
		CtxTimeout: sink.DefaultCtxTimeout,
		Ctx:        context.Background(),
	}

	h, err := sink.NewHTTPSink(sinkOptions)
	if err != nil {
		return nil, fmt.Errorf("creating HTTPSink: %w", err)
	}

	return sdkIntegration.New(i.metadata.Name, i.metadata.Version, sdkIntegration.Writer(h))
}

// k8sConfig returns a Kubernetes rest.Config from either in-cluster variables or from a kubeconfig file if the former
// are not found.
func (i *Integration) k8sConfig(c *config.Config) (*rest.Config, error) {
	i.Logger.Debug("Fetching in-cluster config")
	inclusterConfig, err := rest.InClusterConfig()
	if err == nil {
		return inclusterConfig, nil
	}

	i.Logger.Warnf("Could not get in-cluster config: %v", err)

	i.Logger.Debug("Figuring out Kubeconfig path")
	kubeconf := c.KubeconfigPath
	if kubeconf == "" {
		kubeconf = path.Join(homedir.HomeDir(), ".kube", "config")
		i.Logger.Debugf("Kubeconfig path not defined, using default %q", kubeconf)
	}

	kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeconf)
	if err != nil {
		return nil, fmt.Errorf("could not load local kube config: %w", err)
	}

	i.Logger.Infof("Using kubeconfig file: %q", kubeconf)

	return kubeconfig, nil
}

// Metadata holds information about the integration.
type Metadata struct {
	Name      string
	Version   string
	GitCommit string
	BuildDate string
}

func (m Metadata) String() string {
	return fmt.Sprintf("New Relic %s integration Version: %s, Platform: %s, GoVersion: %s, GitCommit: %s, BuildDate: %s\n",
		strings.Title(strings.Replace(m.Name, "com.newrelic.", "", 1)),
		m.Version,
		fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		runtime.Version(),
		m.GitCommit,
		m.BuildDate,
	)
}
