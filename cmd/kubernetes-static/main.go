package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v2/internal/testutil"
	"github.com/newrelic/nri-kubernetes/v2/src/data"
	ksmClient "github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
	ksmGrouper "github.com/newrelic/nri-kubernetes/v2/src/ksm/grouper"
	kubletClient "github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
	kubeletGrouper "github.com/newrelic/nri-kubernetes/v2/src/kubelet/grouper"
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

	testServer, err := testData.Server()
	if err != nil {
		logrus.Fatalf("Error building testserver: %v", err)
	}

	k8sData, err := testutil.LatestVersion().K8s()
	if err != nil {
		logrus.Fatalf("error instantiating fake k8s objects: %v", err)
	}

	fakeK8s := fake.NewSimpleClientset(k8sData.Everything()...)

	i, err := integration.New(integrationName, integrationVersion, integration.Args(&args))
	if err != nil {
		logrus.Fatal(err)
	}

	logger := log.NewStdErr(args.Verbose)

	nodeGetter, closeChan := discovery.NewNodeLister(fakeK8s)
	defer close(closeChan)

	u, err := url.Parse(testServer.KubeletEndpoint())
	if err != nil {
		log.Fatal(err)
	}

	// Kubelet
	kubeletClient, err := kubletClient.New(kubletClient.StaticConnector(&http.Client{Timeout: time.Minute * 10}, *u))
	if err != nil {
		log.Fatal(err)
	}

	podsFetcher := kubeletmetric.NewPodsFetcher(logger, kubeletClient)
	kubeletGrouper, err := kubeletGrouper.New(
		kubeletGrouper.Config{
			NodeGetter: nodeGetter,
			Client:     kubeletClient,
			Fetchers: []data.FetchFunc{
				podsFetcher.DoPodsFetch,
				kubeletmetric.CadvisorFetchFunc(kubeletClient.MetricFamiliesGetFunc(kubeletmetric.KubeletCAdvisorMetricsPath), metric.CadvisorQueries),
			},
			DefaultNetworkInterface: "ens5",
		}, kubeletGrouper.WithLogger(logger))
	if err != nil {
		log.Fatal(err)
	}

	kc, err := ksmClient.New(ksmClient.WithLogger(log.New(true, os.Stderr)))
	if err != nil {
		log.Fatal(err)
	}

	fakeLister, _ := discovery.NewServicesLister(fakeK8s)
	kg, err := ksmGrouper.New(ksmGrouper.Config{
		MetricFamiliesGetter: kc.MetricFamiliesGetFunc(testServer.KSMEndpoint()),
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

	// TODO add control plane scraper.

	k8sVersion := &version.Info{GitVersion: "v1.18.19"}

	for _, job := range jobs {

		logrus.Infof("Starting job: %s", job.Name)

		result := job.Populate(i, "test-cluster", logger, k8sVersion)

		if result.Populated {
			logrus.Infof("Successfully populated job: %s", job.Name)
		}

		if len(result.Errors) > 0 {
			logrus.Warningf("Job %s ran with errors: %s", job.Name, result.Error())
		}
	}

	if err := i.Publish(); err != nil {
		logrus.Fatalf("Error while publishing: %v", err)
	}

	fmt.Println()
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
