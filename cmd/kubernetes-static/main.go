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
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"

	"github.com/newrelic/nri-kubernetes/v2/src/apiserver"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm"
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

// Embed static metrics into binary.
//go:embed data
var content embed.FS

func main() {
	// Determines which subdirectory of cmd/kubernetes-static/ to use
	// for serving the static metrics
	k8sMetricsVersion := os.Getenv("K8S_METRICS_VERSION")
	if k8sMetricsVersion == "" {
		k8sMetricsVersion = "1_18"
	}

	endpoint := startStaticMetricsServer(content, k8sMetricsVersion)

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
	podsFetcher := kubeletmetric.NewPodsFetcher(logger, kubeletClient, true)
	kubeletGrouper := kubelet.NewGrouper(
		kubeletClient,
		logger,
		apiServerClient,
		"ens5",
		podsFetcher.FetchFuncWithCache(),
		kubeletmetric.CadvisorFetchFunc(kubeletClient, metric.CadvisorQueries))
	// KSM
	ksmClient := newBasicHTTPClient(endpoint + "/ksm")
	k8sClient := &client.MockedKubernetes{}

	serviceList := &v1.ServiceList{
		Items: []v1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-state-metrics",
					Namespace: "kube-system",
				},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"l1": "v1",
						"l2": "v2",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cockroachdb",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"l1": "v1",
						"l2": "v2",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "metrics-server",
					Namespace: "kube-system",
				},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"l1": "v1",
						"l2": "v2",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubernetes",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"l1": "v1",
						"l2": "v2",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-dns",
					Namespace: "kube-system",
				},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"l1": "v1",
						"l2": "v2",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cockroachdb-public",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"l1": "v1",
						"l2": "v2",
					},
				},
			},
		},
	}

	k8sClient.On("ListServices").Return(serviceList, nil)
	ksmGrouper := ksm.NewGrouper(ksmClient, metric.KSMQueries, logger, k8sClient)

	jobs := []*scrape.Job{
		scrape.NewScrapeJob("kubelet", kubeletGrouper, metric.KubeletSpecs),
		scrape.NewScrapeJob("kube-state-metrics", ksmGrouper, metric.KSMSpecs),
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
	fmt.Println("Hosting Mock Metrics data on:", endpoint)

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

func (b basicHTTPClient) Do(method, path string) (*http.Response, error) {
	endpoint := fmt.Sprintf("%s%s", b.url, path)
	log.Info("Getting: %s", endpoint)

	return b.httpClient.Get(endpoint)
}

func (b basicHTTPClient) NodeIP() string {
	return "localhost"
}
