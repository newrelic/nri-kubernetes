package kubelet_test

// This file holds the integration tests for the Kubelet package.

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/testutil"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet"
	kubeletClient "github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestScraper(t *testing.T) {

	commonMetricsToExclude := []string{"net.rxBytesPerSecond", "net.txBytesPerSecond", "net.errorsPerSecond"}
	nodeMetricsToExclude := append(commonMetricsToExclude, "allocatableCpuCoresUtilization", "allocatableMemoryUtilization")
	// Create an asserter with the settings that are shared for all test scenarios.
	asserter := testutil.NewAsserter().
		Using(metric.KubeletSpecs).
		Silently().
		Excluding(
			testutil.ExcludeMetrics("pod", commonMetricsToExclude...),
			testutil.ExcludeMetrics("node", nodeMetricsToExclude...),
			testutil.ExcludeOptional(),
		)

	for _, version := range testutil.AllVersions() {
		t.Run(fmt.Sprintf("for_version_%s", version), func(t *testing.T) {
			t.Parallel()

			testServer, err := version.Server()
			if err != nil {
				t.Fatalf("Cannot create fake kubelet server: %v", err)
			}

			u, _ := url.Parse(testServer.KubeletEndpoint())

			kubeletClient, err := kubeletClient.New(kubeletClient.FixedConnector(&http.Client{}, *u))
			require.NoError(t, err)

			fakeK8s := fake.NewSimpleClientset(testutil.K8sEverything()...)

			scraper, err := kubelet.NewScraper(&config.Mock{
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
