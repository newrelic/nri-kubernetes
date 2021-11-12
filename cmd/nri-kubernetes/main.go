package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/sethgrid/pester"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm"
	ksmClient "github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
	"github.com/newrelic/nri-kubernetes/v2/src/sink"
)

var logger log.Logger

const (
	ExitClients = iota
	ExitIntegration
	ExitLoop
)

type clusterClients struct {
	k8s client.Kubernetes
	ksm ksmClient.MetricFamiliesGetter
}

func main() {
	// TODO: Can this error?
	conf := config.LoadConfig()
	logger = log.NewStdErr(conf.Verbose)

	clients, err := buildClients()
	if err != nil {
		log.Error(err.Error())
		os.Exit(ExitClients)
	}

	i, err := createIntegrationWithHTTPSink()
	if err != nil {
		logger.Errorf("creating integration with http sink: %w", err)
		os.Exit(ExitIntegration)
	}

	// TODO: Here we will switch-case between components: KSM, ControlPlane, etc.
	err = runKSM(&conf, clients, i)
	if err != nil {
		logger.Errorf(err.Error())
		os.Exit(ExitLoop)
	}
}

func buildClients() (*clusterClients, error) {
	k8s, err := client.NewKubernetes(true)
	if err != nil {
		return nil, fmt.Errorf("building kubernetes client: %w", err)
	}

	ksmCli, err := ksmClient.New(ksmClient.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("building KSM client: %w", err)
	}

	return &clusterClients{
		k8s: k8s,
		ksm: ksmCli,
	}, nil
}

func runKSM(config *config.Mock, clients *clusterClients, i *integration.Integration) error {
	ksmScraper, err := ksm.NewScraper(config, ksm.Providers{
		// TODO: Get rid of custom client.Kubernetes wrapper and use kubernetes.Interface directly.
		K8s: clients.k8s.GetClient(),
		KSM: clients.ksm,
	},
		ksm.WithLogger(logger),
	)

	if err != nil {
		return fmt.Errorf("building KSM scraper: %w", err)
	}

	defer ksmScraper.Close()

	for {
		err = ksmScraper.Run(i)
		if err != nil {
			return fmt.Errorf("scraping KSM: %w", err)
		}

		err = i.Publish()
		if err != nil {
			return fmt.Errorf("publishing integration: %w", err)
		}

		time.Sleep(config.Interval)
	}
}

func createIntegrationWithHTTPSink() (*integration.Integration, error) {
	c := pester.New()
	c.Backoff = pester.LinearBackoff
	c.MaxRetries = 5
	c.Timeout = sink.DefaultRequestTimeout
	c.LogHook = func(e pester.ErrEntry) {
		logger.Debugf("sending data to httpSink: %q", e)
	}

	sinkOptions := sink.HTTPSinkOptions{
		URL:        sink.DefaultAgentForwarderEndpoint,
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
