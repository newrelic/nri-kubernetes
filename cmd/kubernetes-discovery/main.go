package main

import (
	"flag"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
	"github.com/sirupsen/logrus"

	k8sclient "github.com/newrelic/nri-kubernetes/v2/src/client"
)

const (
	KSMPodLabel = "ksm_pod_label"
)

var (
	discovery   = flag.String("discovery", KSMPodLabel, "Which discovery mechanism to run")
	ksmPodLabel = flag.String("ksm_pod_label", "my-custom-ksm", "[ksm_pod_label] The label to search for")
)

func main() {
	flag.Parse()

	verbose := true

	logger := log.New(verbose)

	tryLocalKubeconfig := true

	k8sClient, err := k8sclient.NewKubernetes(tryLocalKubeconfig)
	if err != nil {
		logger.Fatalf("Could not create Kubernetes client: %v", err)
	}

	switch *discovery {
	case KSMPodLabel:
		runKSMPodLabel(k8sClient, logger)
	default:
		logger.Infof("Invalid discovery type: %s", *discovery)
	}
}

func runKSMPodLabel(kubernetes k8sclient.Kubernetes, logger *logrus.Logger) {
	discoverer := client.NewPodLabelDiscoverer(*ksmPodLabel, 8080, "http", logger, kubernetes)
	ksm, err := discoverer.Discover(time.Second * 5)
	if err != nil {
		logger.Fatalf("Discovering KSM: %v", err)
	}

	logger.Infof("Found KSM pod on HostIP: %s", ksm.NodeIP())
}
