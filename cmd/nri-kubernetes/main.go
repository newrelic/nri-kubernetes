package main

import (
	"context"
	"fmt"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/sethgrid/pester"
	"k8s.io/apimachinery/pkg/version"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v2/src/apiserver"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
	"github.com/newrelic/nri-kubernetes/v2/src/sink"
	"github.com/newrelic/nri-kubernetes/v2/src/storage"
)

type clientsCluster struct {
	k8s                 client.Kubernetes
	ksm                 ksm.Client
	api                 apiserver.Client
	endpointsDiscoverer discovery.EndpointsDiscoverer
	servicesDiscoverer  discovery.ServiceDiscoverer
}

func main() {
	c := config.LoadConfig()
	logger := log.NewStdErr(c.Verbose)

	k8s, err := client.NewKubernetes(true)
	if err != nil {
		log.Fatal(err)
	}

	apiClient := apiserver.NewFileCacheClientWrapper(apiserver.NewClient(k8s), client.DiscoveryCacherConfig{
		TTL:       3 * time.Hour,
		TTLJitter: 50,
		Storage:   storage.NewJSONDiskStorage("/var/cache/nr-kubernetes/apiserverK8SVersion"),
	})

	ksmDiscoverer, err := getDiscoverer(c, k8s, logger)
	if err != nil {
		log.Fatal(err)
	}

	servicesDiscoverer, err := discovery.NewServicesDiscoverer(k8s.GetClient())
	if err != nil {
		log.Fatal(err)
	}

	ksmClient, err := ksm.NewKSMClient(logger)
	if err != nil {
		log.Fatal(err)
	}

	clients := clientsCluster{
		k8s:                 k8s,
		ksm:                 ksmClient,
		api:                 apiClient,
		endpointsDiscoverer: ksmDiscoverer,
		servicesDiscoverer:  servicesDiscoverer,
	}

	if err := run(c, logger, clients); err != nil {
		log.Fatal(err)
	}
}

func run(c config.Mock, logger log.Logger, clients clientsCluster) error {

	i, err := createIntegrationWithHTTPSink(logger)
	if err != nil {
		return fmt.Errorf("creating integration with http sink: %w", err)
	}

	for {
		k8sVersion, err := clients.api.GetServerVersion()
		if err != nil {
			logger.Errorf("Error getting the kubernetes server version: %v", err)
		}

		err = scrapeKSMEndpoints(c, logger, i, clients, k8sVersion)
		if err != nil {
			return err
		}

		err = i.Publish()
		if err != nil {
			return fmt.Errorf("publishing integration: %w", err)
		}

		time.Sleep(1 * time.Second)
	}
}

func scrapeKSMEndpoints(c config.Mock, logger log.Logger, i *integration.Integration, clients clientsCluster, k8sVersion *version.Info) error {
	populated := false

	endpoints, err := clients.endpointsDiscoverer.Discover()
	if err != nil {
		return fmt.Errorf("discovering KSM endpoints: %w", err)
	}

	logger.Debugf("Discovered endpoints: %q", endpoints)

	services, err := clients.servicesDiscoverer.Discover()
	if err != nil {
		return fmt.Errorf("discovering KSM services: %w", err)
	}

	for _, endpoint := range endpoints {
		ksmGrouperConfig := &ksm.GrouperConfig{
			MetricFamiliesGetter: clients.ksm.MetricFamiliesGetterForEndpoint(endpoint, c.KSMConfig.KubeStateMetricsScheme),
			Logger:               logger,
			Services:             services,
			Queries:              metric.KSMQueries,
		}

		ksmGrouper, err := ksm.NewValidatedGrouper(ksmGrouperConfig)
		if err != nil {
			return fmt.Errorf("creating KSM grouper: %w", err)
		}

		job := scrape.NewScrapeJob("kube-state-metrics", ksmGrouper, metric.KSMSpecs)

		logger.Debugf("Running job: %s", job.Name)

		r := job.Populate(i, c.ClusterName, logger, k8sVersion)
		if r.Errors != nil {
			logger.Debugf("populating KMS: %v", r.Error())
		}

		if r.Populated && !c.KSMConfig.DistributedKubeStateMetrics {
			populated = true
			break
		}
	}

	if !populated {
		return fmt.Errorf("KSM data was not populated after trying all endpoints")
	}

	return nil
}

func createIntegrationWithHTTPSink(logger log.Logger) (*integration.Integration, error) {
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

func getDiscoverer(c config.Mock, k8s client.Kubernetes, logger log.Logger) (discovery.EndpointsDiscoverer, error) {
	var opts []ksm.EndpointDiscoveryOptions

	if c.KSMConfig.KubeStateMetricsURL != "" {
		logger.Debugf("ksm discovery disabled")
		opts = append(opts, ksm.WithFixedEndpoint(c.KSMConfig.KubeStateMetricsURL))
	}

	if c.KSMConfig.KubeStateMetricsNamespace != "" {
		opts = append(opts, ksm.WithNamespace(c.KSMConfig.KubeStateMetricsNamespace))
	}

	if c.KSMConfig.KubeStateMetricsPodLabel != "" {
		opts = append(opts, ksm.WithLabelSelector(c.KSMConfig.KubeStateMetricsPodLabel))
	}

	if c.KSMConfig.KubeStateMetricsPort != 0 {
		opts = append(opts, ksm.WithPort(c.KSMConfig.KubeStateMetricsPort))
	}

	return ksm.NewEndpointsDiscoverer(k8s.GetClient(), opts...)
}
