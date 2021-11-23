package kubelet_test

// This file holds the integration tests for the Kubelet package.

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/log"
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
	asserter := testutil.NewAsserter().
		Using(metric.KubeletSpecs).
		Silently().
		// TODO(roobre): We should not exclude Optional, pod or hpa metrics. To be tackled in a follow-up PR.
		ExcludingOptional().
		Excluding("pod", commonMetricsToExclude...).
		Excluding("node", nodeMetricsToExclude...)

	for _, version := range testutil.AllVersions() {
		t.Run(fmt.Sprintf("for_version_%s", version), func(t *testing.T) {
			t.Parallel()

			testServer, err := version.Server()
			if err != nil {
				t.Fatalf("Cannot create fake kubelet server: %v", err)
			}

			u, _ := url.Parse(testServer.KubeletEndpoint())

			kubeletCli := kubeletClient.NewClientMock(&http.Client{}, *u)
			if err != nil {
				t.Fatalf("error creating kubelet client: %v", err)
			}

			fakeK8s := fake.NewSimpleClientset(testutil.K8sEverything()...)

			scraper, err := kubelet.NewScraper(&config.Mock{
				ClusterName: t.Name(),
			}, kubelet.Providers{
				K8s:      fakeK8s,
				Kubelet:  kubeletCli,
				CAdvisor: kubeletCli,
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
