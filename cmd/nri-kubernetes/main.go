package main

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	sdk "github.com/newrelic/infra-integrations-sdk/integration"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/controlplane"
	"github.com/newrelic/nri-kubernetes/v3/src/integration"
	"github.com/newrelic/nri-kubernetes/v3/src/ksm"
	ksmClient "github.com/newrelic/nri-kubernetes/v3/src/ksm/client"
	"github.com/newrelic/nri-kubernetes/v3/src/kubelet"
	kubeletClient "github.com/newrelic/nri-kubernetes/v3/src/kubelet/client"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

const (
	integrationName = "com.newrelic.kubernetes"

	_ = iota
	exitClients
	exitIntegration
	exitLoop
	exitSetup
)

var (
	integrationVersion = "0.0.0"
	gitCommit          = ""
	buildDate          = ""
)

var logger *log.Logger

type clusterClients struct {
	k8s      kubernetes.Interface
	ksm      prometheus.MetricFamiliesGetFunc
	cAdvisor prometheus.MetricFamiliesGetFunc
	kubelet  client.HTTPGetter
}

func main() {
	logger = log.StandardLogger()

	c, err := config.LoadConfig(config.DefaultFilePath, config.DefaultFileName)
	if err != nil {
		log.Error(err.Error())
		os.Exit(exitIntegration)
	}

	if c.Verbose {
		logger.SetLevel(log.DebugLevel)
	}

	if c.LogLevel != "" {
		level, err := log.ParseLevel(c.LogLevel)
		if err != nil {
			log.Warnf("Cannot parse log level %q: %v", c.LogLevel, err)
		} else {
			logger.SetLevel(level)
		}
	}

	iw, err := integration.NewWrapper(
		integration.WithLogger(logger),
		integration.WithMetadata(integration.Metadata{
			Name:    integrationName,
			Version: integrationVersion,
		}),
	)
	if err != nil {
		logger.Errorf("creating integration wrapper: %v", err)
		os.Exit(exitIntegration)
	}

	i, err := iw.Integration(c.Sink.HTTP)
	if err != nil {
		logger.Errorf("creating integration with http sink: %v", err)
		os.Exit(exitIntegration)
	}

	logger.Infof(
		"New Relic %s integration Version: %s, Platform: %s, GoVersion: %s, GitCommit: %s, BuildDate: %s\n",
		strings.Title(strings.Replace(integrationName, "com.newrelic.", "", 1)),
		integrationVersion,
		fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		runtime.Version(),
		gitCommit,
		buildDate)

	clients, err := buildClients(c)
	if err != nil {
		logger.Errorf("building clients: %v", err)
		os.Exit(exitClients)
	}

	var kubeletScraper *kubelet.Scraper
	if c.Kubelet.Enabled {
		kubeletScraper, err = setupKubelet(c, clients)
		if err != nil {
			logger.Errorf("setting up ksm scraper: %v", err)
			os.Exit(exitSetup)
		}
	}

	var ksmScraper *ksm.Scraper
	if c.KSM.Enabled {
		ksmScraper, err = setupKSM(c, clients)
		if err != nil {
			logger.Errorf("setting up ksm scraper: %v", err)
			os.Exit(exitSetup)
		}
		defer ksmScraper.Close()
	}

	var controlplaneScraper *controlplane.Scraper
	if c.ControlPlane.Enabled {
		controlplaneScraper, err = setupControlPlane(c, clients)
		if err != nil {
			logger.Errorf("setting up control plane scraper: %v", err)
			os.Exit(exitSetup)
		}
		defer controlplaneScraper.Close()
	}

	for {
		start := time.Now()

		logger.Debugf("scraping data from all the scrapers defined: KSM: %t, Kubelet: %t, ControlPlane: %t",
			c.KSM.Enabled, c.Kubelet.Enabled, c.ControlPlane.Enabled)

		// TODO think carefully to the signature of this function
		err := runScrapers(c, ksmScraper, kubeletScraper, controlplaneScraper, i)
		if err != nil {
			logger.Errorf("retrieving scraper data: %v", err)
			os.Exit(exitLoop)
		}

		logger.Debugf("publishing data")
		err = i.Publish()
		if err != nil {
			logger.Errorf("publishing integration: %v", err)
			os.Exit(exitLoop)
		}

		logger.Debugf("waiting %f seconds for next interval", c.Interval.Seconds())

		// Sleep interval minus the time that took to scrape.
		time.Sleep(c.Interval - time.Since(start))
	}
}

func runScrapers(c *config.Config, ksmScraper *ksm.Scraper, kubeletScraper *kubelet.Scraper, controlplaneScraper *controlplane.Scraper, i *sdk.Integration) error {
	if c.KSM.Enabled {
		err := ksmScraper.Run(i)
		if err != nil {
			return fmt.Errorf("retrieving ksm data: %w", err)
		}
	}

	if c.Kubelet.Enabled {
		err := kubeletScraper.Run(i)
		if err != nil {
			return fmt.Errorf("retrieving kubelet data: %w", err)
		}
	}

	if c.ControlPlane.Enabled {
		err := controlplaneScraper.Run(i)
		if err != nil {
			return fmt.Errorf("retrieving control plane data: %w", err)
		}
	}

	return nil
}

func setupKSM(c *config.Config, clients *clusterClients) (*ksm.Scraper, error) {
	providers := ksm.Providers{
		K8s: clients.k8s,
		KSM: clients.ksm,
	}

	ksmScraper, err := ksm.NewScraper(c, providers, ksm.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("building KSM scraper: %w", err)
	}

	return ksmScraper, nil
}

func setupControlPlane(c *config.Config, clients *clusterClients) (*controlplane.Scraper, error) {
	providers := controlplane.Providers{
		K8s: clients.k8s,
	}

	restConfig, err := getK8sConfig(c)
	if err != nil {
		return nil, err
	}

	controlplaneScraper, err := controlplane.NewScraper(
		c,
		providers,
		controlplane.WithLogger(logger),
		controlplane.WithRestConfig(restConfig),
	)
	if err != nil {
		return nil, fmt.Errorf("building KSM scraper: %w", err)
	}

	return controlplaneScraper, nil
}

func setupKubelet(c *config.Config, clients *clusterClients) (*kubelet.Scraper, error) {
	providers := kubelet.Providers{
		K8s:      clients.k8s,
		Kubelet:  clients.kubelet,
		CAdvisor: clients.cAdvisor,
	}
	ksmScraper, err := kubelet.NewScraper(c, providers, kubelet.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("building kubelet scraper: %w", err)
	}

	return ksmScraper, nil
}

func buildClients(c *config.Config) (*clusterClients, error) {
	k8sConfig, err := getK8sConfig(c)
	if err != nil {
		return nil, fmt.Errorf("retrieving k8s config: %w", err)
	}

	k8s, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("building kubernetes client: %w", err)
	}

	var ksmCli *ksmClient.Client
	if c.KSM.Enabled {
		ksmCli, err = ksmClient.New(
			ksmClient.WithLogger(logger),
			ksmClient.WithTimeout(c.KSM.Timeout),
			ksmClient.WithMaxRetries(c.KSM.Retries),
		)
		if err != nil {
			return nil, fmt.Errorf("building KSM client: %w", err)
		}
	}

	var kubeletCli *kubeletClient.Client
	if c.Kubelet.Enabled {
		kubeletCli, err = kubeletClient.New(
			kubeletClient.DefaultConnector(k8s, c, k8sConfig, logger),
			kubeletClient.WithLogger(logger),
			kubeletClient.WithMaxRetries(c.Kubelet.Retries),
		)
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

func getK8sConfig(c *config.Config) (*rest.Config, error) {
	inclusterConfig, err := rest.InClusterConfig()
	if err == nil {
		return inclusterConfig, nil
	}
	logger.Warnf("collecting in cluster config: %v", err)

	kubeconf := c.KubeconfigPath
	if kubeconf == "" {
		kubeconf = path.Join(homedir.HomeDir(), ".kube", "config")
	}

	inclusterConfig, err = clientcmd.BuildConfigFromFlags("", kubeconf)
	if err != nil {
		return nil, fmt.Errorf("could not load local kube config: %w", err)
	}

	logger.Warnf("using local kube config: %q", kubeconf)

	return inclusterConfig, nil
}
