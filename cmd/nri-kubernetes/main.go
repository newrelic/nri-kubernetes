package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

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
	_, err := restConfig()
	if err != nil {
		return fmt.Errorf("getting REST config: %w", err)
	}

	discoverer, err := ksm.NewDiscoverer()
	if err != nil {
		return fmt.Errorf("creating KSM discoverer: %w", err)
	}

	endpoints, err := discoverer.Discover()
	if err != nil {
		return fmt.Errorf("discovering KSM endpoints: %w", err)
	}

	verbose := true
	logger := log.NewStdErr(verbose)
	var clientTimeout time.Duration

	for _, endpoint := range endpoints {
		metricFamiliesGetter, err := ksmclient.NewKSMClient(clientTimeout, endpoint, logger)
		if err != nil {
			return fmt.Errorf("creating KSM client: %w", err)
		}

		k8sClient, err := client.NewKubernetes(true)
		if err != nil {
			return fmt.Errorf("creating Kubernetes client: %w", err)
		}

		ksmGrouperConfig := &ksm.GrouperConfig{
			MetricFamiliesGetter: metricFamiliesGetter,
			Logger:               logger,
			K8sClient:            k8sClient,
			Queries:              metric.KSMQueries,
		}

		grouper, err := ksm.NewValidatedGrouper(ksmGrouperConfig)
		if err != nil {
			return fmt.Errorf("creating KSM grouper: %w", err)
		}

		// TODO: What does Grouper abstraction mean?
		rawGroups, err := grouper.Group(metric.KSMSpecs)
		if err != nil {
			logger.Warnf("Grouping returned error: %v", err)
		}

		fmt.Println(rawGroups)
	}

	return nil
}

func restConfig() (*rest.Config, error) {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if home := homedir.HomeDir(); kubeconfigPath == "" && home != "" {
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	c, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("building config: %w", err)
	}

	return c, nil
}
