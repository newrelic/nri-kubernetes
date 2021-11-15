package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/version"

	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v2/internal/testutil"
	"github.com/newrelic/nri-kubernetes/v2/src/apiserver"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane"
	ksmClient "github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
	ksmGrouper "github.com/newrelic/nri-kubernetes/v2/src/ksm/grouper"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet"
	kubeletmetric "github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
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
	// Determines which subdirectory of cmd/kubernetes-static/ to use
	// for serving the static metrics
	testData := testutil.LatestVersion()
	if envVersion := os.Getenv("K8S_METRICS_VERSION"); envVersion != "" {
		testData = testutil.Version(envVersion)
	}

	testSever, err := testData.Server()
	if err != nil {
		logrus.Fatalf("Error building testserver: %v", err)
	}

	fakeK8s, err := testutil.FakeK8sClient()
	if err != nil {
		logrus.Fatal(err)
	}

	integration, err := integration.New(integrationName, integrationVersion, integration.Args(&args))
	if err != nil {
		logrus.Fatal(err)
	}

	logger := log.NewStdErr(args.Verbose)

	// ApiServer
	apiServerClient := apiserver.TestAPIServer{Mem: map[string]*apiserver.NodeInfo{
		// this nodename should be the same as the ones in the data folder
		"minikube": {
			NodeName: "minikube",
			Labels: map[string]string{
				"node-role.kubernetes.io/master": "",
			},
			Conditions: []v1.NodeCondition{
				{
					Type:   "DiskPressure",
					Status: v1.ConditionFalse,
				},
				{
					Type:   "MemoryPressure",
					Status: v1.ConditionFalse,
				},
				{
					Type:   "DiskPressure",
					Status: v1.ConditionFalse,
				},
				{
					Type:   "PIDPressure",
					Status: v1.ConditionFalse,
				},
				{
					Type:   "Ready",
					Status: v1.ConditionTrue,
				},
			},
			Unschedulable: false,
		},
	}}

	// Kubelet
	kubeletClient := newBasicHTTPClient(testSever.KubeletEndpoint())
	podsFetcher := kubeletmetric.NewPodsFetcher(logger, kubeletClient)
	kubeletGrouper := kubelet.NewGrouper(
		kubeletClient,
		logger,
		apiServerClient,
		"ens5",
		podsFetcher.FetchFuncWithCache(),
		kubeletmetric.CadvisorFetchFunc(kubeletClient, metric.CadvisorQueries),
	)

	kc, err := ksmClient.New(ksmClient.WithLogger(log.New(true, os.Stderr)))
	if err != nil {
		log.Fatal(err)
	}

	fakeLister, _ := discovery.NewServicesLister(fakeK8s)
	kg, err := ksmGrouper.New(ksmGrouper.Config{
		MetricFamiliesGetter: kc.MetricFamiliesGetter(testSever.KSMEndpoint()),
		Queries:              metric.KSMQueries,
		ServicesLister:       fakeLister,
	}, ksmGrouper.WithLogger(logger))
	if err != nil {
		log.Fatal(err)
	}

	jobs := []*scrape.Job{
		scrape.NewScrapeJob("kubelet", kubeletGrouper, metric.KubeletSpecs),
		scrape.NewScrapeJob("kube-state-metrics", kg, metric.KSMSpecs),
	}

	// controlPlaneComponentPods maps component.Name to the pod name
	// found in the file `cmd/kubernetes-static/data/kubelet/pods`
	controlPlaneComponentPods := map[controlplane.ComponentName]string{
		controlplane.Scheduler:         "kube-scheduler-minikube",
		controlplane.Etcd:              "etcd-minikube",
		controlplane.ControllerManager: "kube-controller-manager-minikube",
		controlplane.APIServer:         "kube-apiserver-minikube",
	}

	for _, component := range controlplane.BuildComponentList() {
		componentGrouper := controlplane.NewComponentGrouper(
			newBasicHTTPClient(testSever.ControlPlaneEndpoint(string(component.Name))),
			component.Queries,
			logger,
			controlPlaneComponentPods[component.Name],
		)
		jobs = append(
			jobs,
			scrape.NewScrapeJob(string(component.Name), componentGrouper, component.Specs),
		)
	}

	k8sVersion := &version.Info{GitVersion: "v1.18.19"}

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

	fmt.Println()
}

func startStaticMetricsServer(content embed.FS, k8sMetricsVersion string) string {
	listenAddress := "127.0.0.1:0"
	// This will allocate a random port
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		logrus.Fatalf("Error listening on %q: %v", listenAddress, err)
	}

	endpoint := fmt.Sprintf("http://%s", listener.Addr())
	logrus.Infof("Hosting Mock Metrics data on %s", endpoint)

	mux := http.NewServeMux()

	path := filepath.Join("data", k8sMetricsVersion)
	k8sContent, err := fs.Sub(content, path)
	if err != nil {
		logrus.Fatalf("Error taking a %q subtree of embedded data: %v", path, err)
	}

	mux.Handle("/", http.FileServer(http.FS(k8sContent)))
	go func() {
		logrus.Fatal(http.Serve(listener, mux))
	}()

	return endpoint
}

func newBasicHTTPClient(url string) *basicHTTPClient {
	return &basicHTTPClient{
		url: url,
		httpClient: http.Client{
			Timeout: time.Minute * 10, // high for debugging purposes
		},
	}
}

type basicHTTPClient struct {
	url        string
	httpClient http.Client
}

func (b basicHTTPClient) Get(path string) (*http.Response, error) {
	endpoint := fmt.Sprintf("%s%s", b.url, path)
	log.Info("Getting: %s", endpoint)

	return b.httpClient.Get(endpoint)
}

func (b basicHTTPClient) NodeIP() string {
	return "localhost"
}
