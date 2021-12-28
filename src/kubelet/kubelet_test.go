package kubelet_test

// This file holds the integration tests for the Kubelet package.

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v2/internal/logutil"

	"github.com/newrelic/nri-kubernetes/v2/internal/testutil/asserter"
	"github.com/newrelic/nri-kubernetes/v2/internal/testutil/asserter/exclude"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/testutil"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet"
	kubeletClient "github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
)

func TestScraper(t *testing.T) {
	networkMetrics := []string{"net.rxBytesPerSecond", "net.txBytesPerSecond", "net.errorsPerSecond"}
	nodeUtilizationMetrics := []string{"allocatableCpuCoresUtilization", "allocatableMemoryUtilization"}

	runningMetrics := []string{"memoryUsedBytes", "memoryWorkingSetBytes", "cpuUsedCores",
		"fsAvailableBytes", "fsCapacityBytes", "fsUsedBytes", "fsUsedPercent", "fsInodesFree", "fsInodes",
		"fsInodesUsed", "containerID", "containerImageID", "isReady", "podIP"}

	utilizationDependencies := map[string][]string{
		"cpuLimitCores":        {"cpuCoresUtilization"},
		"cpuRequestedCores":    {"requestedCpuCoresUtilization"},
		"memoryLimitBytes":     {"memoryUtilization"},
		"memoryRequestedBytes": {"requestedMemoryUtilization"},
	}

	limitsRequestedRegex := regexp.MustCompile("(Limit|Requested)(Cores|Bytes)")

	// Create an asserter with the settings that are shared for all test scenarios.
	asserter := asserter.New().
		Using(metric.KubeletSpecs).
		Excluding(
			// Common metrics.
			exclude.Metrics(networkMetrics...),
			// Node metrics.
			exclude.Exclude(exclude.Group("node"), exclude.Metrics(nodeUtilizationMetrics...)),
			// Exclude metrics that depend on limits when those limits are not set.
			exclude.Exclude(exclude.Group("pod"), exclude.Dependent(utilizationDependencies)),
			// Exclude metrics known to be missing for pods that are pending.
			exclude.Exclude(
				func(group string, _ *definition.Spec, ent *integration.Entity) bool {
					if group == "pod" || group == "container" {
						return len(ent.Metrics) > 0 && !strings.EqualFold(ent.Metrics[0].Metrics["status"].(string), "running")
					}
					return false
				},
				exclude.Metrics(runningMetrics...),
			),
			// Exclude limits/requested metrics for containers
			func(group string, spec *definition.Spec, e *integration.Entity) bool {
				return group == "container" && limitsRequestedRegex.MatchString(spec.Name)
			},
			// Optional metrics.
			exclude.Optional(),
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
			}, kubelet.WithLogger(logutil.Debug))
			if err != nil {
				t.Fatalf("creating scraper: %v", err)
			}

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
