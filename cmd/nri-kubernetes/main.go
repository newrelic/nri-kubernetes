package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm"
	ksmclient "github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	k8s, err := newClient()
	if err != nil {
		return fmt.Errorf("getting REST config: %w", err)
	}

	// Arbitrary value, same used in Prometheus.
	resyncDuration := 10 * time.Minute

	verbose := true
	logger := log.NewStdErr(verbose)

	stopCh := make(chan struct{})

	discoveryConfig := ksm.DiscoveryConfig{
		EndpointsDiscoveryConfig: discovery.EndpointsDiscoveryConfig{
			EndpointsLister: func(options ...informers.SharedInformerOption) discovery.EndpointsLister {
				factory := informers.NewSharedInformerFactoryWithOptions(k8s, resyncDuration, options...)

				lister := factory.Core().V1().Endpoints().Lister()

				factory.Start(stopCh)
				factory.WaitForCacheSync(stopCh)

				return lister
			},
		},
	}

	discoverer, err := ksm.NewDiscoverer(discoveryConfig)
	if err != nil {
		return fmt.Errorf("creating KSM discoverer: %w", err)
	}

	endpoints, err := discoverer.Discover()
	if err != nil {
		return fmt.Errorf("discovering KSM endpoints: %w", err)
	}

	var noClientTimeout time.Duration

	k8sClient, err := client.NewKubernetes(true)
	if err != nil {
		return fmt.Errorf("creating Kubernetes client: %w", err)
	}

	ksmClient, err := ksmclient.NewKSMClient(noClientTimeout, logger)
	if err != nil {
		return fmt.Errorf("creating KSM client: %w", err)
	}

	for _, endpoint := range endpoints {
		ksmGrouperConfig := &ksm.GrouperConfig{
			MetricFamiliesGetter: ksmClient.MetricFamiliesGetterForEndpoint(endpoint),
			Logger:               logger,
			K8sClient:            k8sClient,
			Queries:              metric.KSMQueries,
		}

		grouper, err := ksm.NewValidatedGrouper(ksmGrouperConfig)
		if err != nil {
			return fmt.Errorf("creating KSM grouper: %w", err)
		}

		// TODO: What does Grouper abstraction mean? Rename it to RawGroupsFetcher?
		rawGroups, err := grouper.Group(metric.KSMSpecs)
		if err != nil {
			logger.Warnf("Grouping returned error: %v", err)
		}

		fmt.Println(rawGroups)
	}

	return nil
}

func newClient() (kubernetes.Interface, error) {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if home := homedir.HomeDir(); kubeconfigPath == "" && home != "" {
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("building config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	return client, nil
}
