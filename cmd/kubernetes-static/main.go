package main

import (
	"fmt"
	"net"
	"net/http"
	"time"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/newrelic/nri-kubernetes/src/apiserver"
	"github.com/newrelic/nri-kubernetes/src/controlplane"
	"github.com/newrelic/nri-kubernetes/src/ksm"
	"github.com/newrelic/nri-kubernetes/src/kubelet"
	metric2 "github.com/newrelic/nri-kubernetes/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/src/metric"
	"github.com/newrelic/nri-kubernetes/src/scrape"
	"github.com/sirupsen/logrus"
)

const (
	integrationName    = "kubernetes-static"
	integrationVersion = "static-local"
)

type argumentList struct {
	sdkArgs.DefaultArgumentList
}

var args argumentList

func main() {

	endpoint := startStaticMetricsServer()
	// let the http server start...
	time.Sleep(time.Millisecond * 100)

	integration, err := sdk.NewIntegrationProtocol2(integrationName, integrationVersion, &args)
	if err != nil {
		logrus.Fatal(err)
	}

	logger := log.New(args.Verbose)

	// ApiServer
	apiServerClient := apiserver.TestAPIServer{Mem: map[string]*apiserver.NodeInfo{
		// this nodename should be the same as the ones in the data folder
		"minikube": {
			NodeName: "minikube",
			Labels: map[string]string{
				"app.kubernetes.io/master": "true",
			},
		},
	}}

	// Kubelet
	kubeletClient := newBasicHTTPClient(endpoint + "/kubelet")
	podsFetcher := metric2.NewPodsFetcher(logger, kubeletClient)
	kubeletGrouper := kubelet.NewGrouper(kubeletClient, logger, apiServerClient,
		podsFetcher.FetchFuncWithCache(),
		metric2.CadvisorFetchFunc(kubeletClient, metric.CadvisorQueries))
	// KSM
	ksmClient := newBasicHTTPClient(endpoint + "/ksm")
	ksmGrouper := ksm.NewGrouper(ksmClient, metric.KSMQueries, logger)

	jobs := []*scrape.Job{
		scrape.NewScrapeJob("kubelet", kubeletGrouper, metric.KubeletSpecs),
		scrape.NewScrapeJob("kube-state-metrics", ksmGrouper, metric.KSMSpecs),
	}

	// controlPlaneComponentPods maps component.Name to the pod name
	// found in the file `cmd/kubernetes-static/data/kubelet/pods`
	controlPlaneComponentPods := map[controlplane.ComponentName]string{
		"scheduler":          "kube-scheduler-minikube",
		"etcd":               "etcd-minikube",
		"controller-manager": "kube-controller-manager-minikube",
		"apiserver":          "kube-apiserver-minikube",
	}

	for _, component := range controlplane.BuildComponentList() {
		componentGrouper := controlplane.NewComponentGrouper(
			newBasicHTTPClient(fmt.Sprintf("%s/controlplane/%s", endpoint, component.Name)),
			component.Queries,
			logger,
			controlPlaneComponentPods[component.Name],
		)
		jobs = append(
			jobs,
			scrape.NewScrapeJob(string(component.Name), componentGrouper, component.Specs),
		)
	}

	for _, job := range jobs {

		logrus.Infof("Starting job: %s", job.Name)
		result := job.Populate(integration, "test-cluster", logger)

		if result.Populated {
			logrus.Infof("Successfully populated job: %s", job.Name)
		}

		if len(result.Errors) > 0 {
			logrus.Warningf("Job %s ran with errors: %s", job.Name, result.Error())
		}
	}

	if err := integration.Publish(); err != nil {
		logrus.Fatalf("Error while publishing: %v", err)
	}
}

func startStaticMetricsServer() string {
	// This will allocate a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	endpoint := fmt.Sprintf("http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)
	fmt.Println("Hosting Mock Metrics data on:", endpoint)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./data")))
	go func() {
		logrus.Fatal(http.Serve(listener, mux))
	}()

	return endpoint
}
