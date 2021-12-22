package kubelet_test

// This file holds the integration tests for the Kubelet package.

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v2/internal/testutil/asserter"
	"github.com/newrelic/nri-kubernetes/v2/internal/testutil/asserter/exclude"

	"github.com/newrelic/nri-kubernetes/v2/src/definition"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/testutil"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet"
	kubeletClient "github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
)

func TestScraper(t *testing.T) {

	commonMetricsToExclude := []string{"net.rxBytesPerSecond", "net.txBytesPerSecond", "net.errorsPerSecond"}
	nodeMetricsToExclude := append(commonMetricsToExclude, "allocatableCpuCoresUtilization", "allocatableMemoryUtilization")

	// Create an asserter with the settings that are shared for all test scenarios.
	asserter := asserter.New().
		Using(metric.KubeletSpecs).
		Excluding(
			exclude.MetricsGroup("pod", commonMetricsToExclude...),
			exclude.MetricsGroup("node", nodeMetricsToExclude...),
			exclude.Optional(),
			ExcludeMissingMetricsPendingPod,
		)

	for _, v := range testutil.AllVersions() {
		// Make a copy of the version variable to use it concurrently
		version := v

		t.Run(fmt.Sprintf("for_version_%s", version), func(t *testing.T) {
			t.Parallel()

			testServer, err := version.Server()
			if err != nil {
				t.Fatalf("Cannot create fake kubelet server: %v", err)
			}

			u, _ := url.Parse(testServer.KubeletEndpoint())

			kubeletClient, err := kubeletClient.New(kubeletClient.StaticConnector(&http.Client{}, *u))
			require.NoError(t, err)

			k8sData, err := version.K8s()
			if err != nil {
				t.Fatalf("error instantiating fake k8s objects: %v", err)
			}

			fakeK8s := fake.NewSimpleClientset(k8sData.Everything()...)

			scraper, err := kubelet.NewScraper(&config.Config{
				ClusterName: t.Name(),
			}, kubelet.Providers{
				K8s:      fakeK8s,
				Kubelet:  kubeletClient,
				CAdvisor: kubeletClient,
			}, kubelet.WithLogger(log.NewStdErr(true)))

			i := testutil.NewIntegration(t)

			err = scraper.Run(i)
			if err != nil {
				t.Fatalf("running scraper: %v", err)
			}

			// Call the asserter for the entities of this particular sub-test.
			asserter.On(i.Entities).Assert(t)
		})
	}
}

func ExcludeMissingMetricsPendingPod(_ string, spec *definition.Spec, ent *integration.Entity) bool {
	metricsToExcludeForPendingPods := []string{"memoryUsedBytes", "memoryWorkingSetBytes", "cpuUsedCores",
		"fsAvailableBytes", "fsCapacityBytes", "fsUsedBytes", "fsUsedPercent", "fsInodesFree", "fsInodes",
		"fsInodesUsed", "containerID", "containerImageID", "isReady"}

	for _, metricSet := range ent.Metrics {
		if metricSet.Metrics["status"] == "Waiting" {
			for _, m := range metricsToExcludeForPendingPods {
				if m == spec.Name {
					return true
				}
			}
		}
	}
	return false
}
