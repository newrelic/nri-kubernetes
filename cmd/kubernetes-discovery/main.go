package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"

	"github.com/newrelic/nri-kubernetes/v2/src/ksm/client"

	k8sclient "github.com/newrelic/nri-kubernetes/v2/src/client"
)

const (
	KSMPodLabel = "ksm_pod_label"
)

var (
	discovery       = flag.String("discovery", KSMPodLabel, "Which discovery mechanism to run")
	ksmPodLabel     = flag.String("ksm_pod_label", "my-custom-ksm", "[ksm_pod_label] The label to search for")
	ksmPodNamespace = flag.String("ksm_pod_namespace", "", "Namespace to query the KSM pod. By default, all namespaces will be queried")
)

func main() {
	flag.Parse()

	verbose := true

	logger := log.NewStdErr(verbose)

	tryLocalKubeconfig := true

	k8sClient, err := k8sclient.NewKubernetes(tryLocalKubeconfig)
	if err != nil {
		logger.Errorf("Could not create Kubernetes client: %v", err)
		os.Exit(1)
	}

	switch *discovery {
	case KSMPodLabel:
		err = runKSMPodLabel(k8sClient, logger)
		if err != nil {
			logger.Errorf("Error %v", err)
			os.Exit(1)
		}
	default:
		logger.Infof("Invalid discovery type: %s", *discovery)
	}
}

func runKSMPodLabel(kubernetes k8sclient.Kubernetes, logger log.Logger) error {
	config := client.PodLabelDiscovererConfig{
		KSMPodLabel:  *ksmPodLabel,
		KSMPodPort:   8080,
		KSMScheme:    "http",
		KSMNamespace: *ksmPodNamespace,
		Logger:       logger,
		K8sClient:    kubernetes,
	}

	discoverer, err := client.NewPodLabelDiscoverer(config)
	if err != nil {
		return fmt.Errorf("initializing discoverer: %w", err)
	}

	ksm, err := discoverer.Discover(time.Second * 5)
	if err != nil {
		return fmt.Errorf("discovering KSM: %w", err)
	}

	logger.Infof("Found KSM pod on HostIP: %s", ksm.NodeIP())

	return nil
}
