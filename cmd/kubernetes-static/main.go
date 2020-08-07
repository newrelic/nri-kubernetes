package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/newrelic/nri-kubernetes/src/apiserver"
	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/controlplane"
	"github.com/newrelic/nri-kubernetes/src/ksm"
	"github.com/newrelic/nri-kubernetes/src/kubelet"
	metric2 "github.com/newrelic/nri-kubernetes/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/src/metric"
	"github.com/newrelic/nri-kubernetes/src/scrape"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/version"
)

const (
	integrationName    = "kubernetes-static"
	integrationVersion = "static-local"

	// set to false to use Kubernetes 1.15 metrics (cAdvisor changes)
	useKubernetes1_16 = false
	useKubernetes1_18 = true
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
				"node-role.kubernetes.io/master": "",
			},
		},
	}}

	// Kubelet
	kubeletClient := newBasicHTTPClient(endpoint + "/kubelet")
	podsFetcher := metric2.NewPodsFetcher(logger, kubeletClient, true)
	kubeletGrouper := kubelet.NewGrouper(
		kubeletClient,
		logger,
		apiServerClient,
		"ens5",
		podsFetcher.FetchFuncWithCache(),
		metric2.CadvisorFetchFunc(kubeletClient, metric.CadvisorQueries))
	// KSM
	ksmClient := newBasicHTTPClient(endpoint + "/ksm")
	k8sClient := new(client.MockedKubernetes)
	serviceList := &v1.ServiceList{
		Items: []v1.Service{
			{
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"l1": "v1",
						"l2": "v2",
					},
				},
			},
		},
	}
	serviceList.Items[0].Namespace = "kube-system"
	serviceList.Items[0].Name = "kube-state-metrics"
	k8sClient.On("ListServices").Return(serviceList, nil)
	ksmGrouper := ksm.NewGrouper(ksmClient, metric.KSMQueries, logger, k8sClient)

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

	k8sVersion := &version.Info{GitVersion: "v1.15.42"}

	for _, job := range jobs {

		logrus.Infof("Starting job: %s", job.Name)

		result := job.Populate(integration, "test-cluster", logger, k8sVersion)

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

	dataDir := "./cmd/kubernetes-static/data/1_15"
	if useKubernetes1_16 {
		dataDir = "./cmd/kubernetes-static/data/1_16"
	}
	if useKubernetes1_18 {
		dataDir = "./cmd/kubernetes-static/data/1_18"
	}

	path, err := filepath.Abs(dataDir)
	if err != nil {
		log.Fatal(errors.New("cannot start server"))
	}

	mux.Handle("/", http.FileServer(http.Dir(path)))
	go func() {
		logrus.Fatal(http.Serve(listener, mux))
	}()

	return endpoint
}
