package main

import (
	"flag"
	"time"

	k8sclient "github.com/newrelic/nri-kubernetes/src/client"

	"github.com/newrelic/nri-kubernetes/src/ksm/client"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/sirupsen/logrus"
)

var discovery = flag.String("discovery", KSMPodLabel, "Which discovery mechanism to run")

var logger = log.New(true)

const (
	KSMPodLabel = "ksm_pod_label"
)

func main() {

	flag.Parse()

	k8sClient, err := k8sclient.NewKubernetes( /* tryLocalKubeconfig */ true)
	if err != nil {
		logrus.Fatalf("could not create kubernetes client: %v", err)
	}

	switch *discovery {
	case KSMPodLabel:
		runKSMPodLabel(k8sClient)
	default:
		logrus.Infof("Invalid discovery type: %s", *discovery)
	}
}

var ksmPodLabel = flag.String("ksm_pod_label", "my-custom-ksm", "[ksm_pod_label] The label to search for")

func runKSMPodLabel(kubernetes k8sclient.Kubernetes) {
	discoverer := client.NewPodLabelDiscoverer(*ksmPodLabel, 8080, "http", logger, kubernetes)
	ksm, err := discoverer.Discover(time.Second * 5)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("found KSM pod on HostIP: %s", ksm.NodeIP())
}
