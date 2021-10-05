package main

import (
	"flag"
	"os"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/sirupsen/logrus"

	k8sclient "github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
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

	logger := log.New(verbose)

	tryLocalKubeconfig := true

	k8sClient, err := k8sclient.NewKubernetes(tryLocalKubeconfig)
	if err != nil {
		logger.Errorf("Could not create Kubernetes client: %v", err)
		os.Exit(1)
	}

	switch *discovery {
	case KSMPodLabel:
		runKSMPodLabel(k8sClient, logger)
	default:
		logger.Infof("Invalid discovery type: %s", *discovery)
	}
}

func runKSMPodLabel(kubernetes k8sclient.Kubernetes, logger *logrus.Logger) {
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
		logger.Fatalf("Initializing discoverer: %v", err)
	}

	ksm, err := discoverer.Discover(time.Second * 5)
	if err != nil {
		logger.Fatalf("Discovering KSM: %v", err)
	}

	logger.Infof("Found KSM pod on HostIP: %s", ksm.NodeIP())
}
