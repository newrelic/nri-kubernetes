package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/sethgrid/pester"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/deprecated"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm"
	ksmClient "github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet"
	kubeletClient "github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
	"github.com/newrelic/nri-kubernetes/v2/src/sink"
)

var logger log.Logger

const (
	_ = iota
	exitClients
	exitIntegration
	exitLoop
	exitSetup
)

type clusterClients struct {
	k8s      kubernetes.Interface
	ksm      prometheus.MetricFamiliesGetFunc
	cAdvisor prometheus.MetricFamiliesGetFunc
	kubelet  client.HTTPGetter
}

func main() {
	c := config.LoadConfig()
	logger = log.NewStdErr(c.Verbose)

	i, err := createIntegrationWithHTTPSink(c.HTTPServerPort)
	if err != nil {
		logger.Errorf("creating integration with http sink: %w", err)
		os.Exit(exitIntegration)
	}

	clients, err := buildClients(c)
	if err != nil {
		log.Error(err.Error())
		os.Exit(exitClients)
	}

	var kubeletScraper *kubelet.Scraper
	if c.Kubelet.Enabled {
		kubeletScraper, err = setupKubelet(c, clients)
		if err != nil {
			logger.Errorf("setting up ksm scraper: %w", err)
			os.Exit(exitSetup)
		}
	}

	var ksmScraper *ksm.Scraper
	if c.KSM.Enabled {
		ksmScraper, err = setupKSM(c, clients)
		if err != nil {
			logger.Errorf("setting up ksm scraper: %w", err)
			os.Exit(exitSetup)
		}
		defer ksmScraper.Close()
	}

	for {
		// TODO think carefully to the signature of this function
		err := runScrapers(c, ksmScraper, kubeletScraper, i, clients)
		if err != nil {
			logger.Errorf("retrieving scraper data: %v", err)
			os.Exit(exitLoop)
		}

		err = i.Publish()
		if err != nil {
			logger.Errorf("publishing integration: %v", err)
			os.Exit(exitLoop)
		}

		time.Sleep(c.Interval)
	}
}

func runScrapers(c *config.Mock, ksmScraper *ksm.Scraper, kubeletScraper *kubelet.Scraper, i *integration.Integration, clients *clusterClients) error {
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
		// TODO this is merely a stub running old code
		err := deprecated.RunControlPlane(c, clients.k8s, i)
		if err != nil {
			return fmt.Errorf("retrieving control-plane data: %w", err)

		}
	}

	return nil
}

func setupKSM(c *config.Mock, clients *clusterClients) (*ksm.Scraper, error) {
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

func setupKubelet(c *config.Mock, clients *clusterClients) (*kubelet.Scraper, error) {
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

func buildClients(c *config.Mock) (*clusterClients, error) {
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
		ksmCli, err = ksmClient.New(ksmClient.WithLogger(logger))
		if err != nil {
			return nil, fmt.Errorf("building KSM client: %w", err)
		}
	}

	var kubeletCli *kubeletClient.Client
	if c.Kubelet.Enabled || c.ControlPlane.Enabled {
		kubeletCli, err = kubeletClient.New(k8s, c, k8sConfig, kubeletClient.WithLogger(logger))
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

func createIntegrationWithHTTPSink(httpServerPort string) (*integration.Integration, error) {
	c := pester.New()
	c.Backoff = pester.LinearBackoff
	c.MaxRetries = 5
	c.Timeout = sink.DefaultRequestTimeout
	c.LogHook = func(e pester.ErrEntry) {
		logger.Debugf("sending data to httpSink: %q", e)
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

	return integration.New("com.newrelic.kubernetes", "test-ksm", integration.Writer(h))
}

func getK8sConfig(tryLocalKubeConfig bool, c config.Mock) (*rest.Config, error) {
	inclusterConfig, err := rest.InClusterConfig()
	if err == nil || !tryLocalKubeConfig {
		return inclusterConfig, nil
	}

	kubeconf := c.KubeconfigPath
	if kubeconf == "" {
		kubeconf = path.Join(homedir.HomeDir(), ".kube", "config")
	}

	inclusterConfig, err = clientcmd.BuildConfigFromFlags("", kubeconf)
	if err != nil {
		return nil, fmt.Errorf("could not load local kube config: %w", err)
	}

	return inclusterConfig, nil
}
