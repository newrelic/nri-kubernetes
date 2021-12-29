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
	// Create an asserter with the settings that are shared for all test scenarios.
	asserter := asserter.New().
		Silently().
		Using(metric.KubeletSpecs).
		Excluding(kubeletExclusions()...)

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

// kubeletExclusions is a helper that returns all the exclusions needed to assert the kubelet metrics without getting
// false negatives.
func kubeletExclusions() []exclude.Func {
	// Network metrics are known to be missing on some environments.
	networkMetrics := []string{"net.rxBytesPerSecond", "net.txBytesPerSecond", "net.errorsPerSecond"}

	// TODO: Unclear why we need to exclude node utilization metrics.
	nodeUtilizationMetrics := []string{"allocatableCpuCoresUtilization", "allocatableMemoryUtilization"}

	// Pods and containers that are not in a running state will not have these metrics.
	notRunningMetrics := []string{"memoryUsedBytes", "memoryWorkingSetBytes", "cpuUsedCores",
		"fsAvailableBytes", "fsCapacityBytes", "fsUsedBytes", "fsUsedPercent", "fsInodesFree", "fsInodes",
		"fsInodesUsed", "containerMemoryMappedFileBytes", "containerID", "containerImageID", "isReady", "podIP"}

	// Utilization metrics will not be present if the corresponding limit/request is not present.
	utilizationDependencies := map[string][]string{
		"cpuLimitCores":        {"cpuCoresUtilization"},
		"cpuRequestedCores":    {"requestedCpuCoresUtilization"},
		"memoryLimitBytes":     {"memoryUtilization"},
		"memoryRequestedBytes": {"requestedMemoryUtilization"},
	}

	// Regex to match limits/requests for CPU and Memory.
	limitsRequestsRegex := regexp.MustCompile("(Limit|Requested)(Cores|Bytes)$")

	return []exclude.Func{
		// Network metrics
		exclude.Metrics(networkMetrics...),

		// Node utilization  metrics.
		exclude.Exclude(exclude.Groups("node"), exclude.Metrics(nodeUtilizationMetrics...)),

		// Exclude limits/requested metrics for nodes, pods and containers
		exclude.Exclude(
			exclude.Groups("node", "pod", "container"),
			func(group string, spec *definition.Spec, e *integration.Entity) bool {
				return limitsRequestsRegex.MatchString(spec.Name)
			},
		),
		// Exclude metrics that depend on limits when those limits are not set.
		exclude.Exclude(exclude.Groups("pod", "container"), exclude.Dependent(utilizationDependencies)),

		// Static pods, typically living in kube-system, do not have creation dates.
		exclude.Exclude(
			exclude.Groups("pod"),
			func(_ string, _ *definition.Spec, ent *integration.Entity) bool {
				return asserter.EntityMetricIs(ent, "namespace", "kube-system")
			},
			exclude.Metrics("createdAt", "createdBy", "createdKind", "deploymentName"),
		),

		// Exclude deploymentName metric for pods not created by a deployment
		exclude.Exclude(
			exclude.Groups("pod", "container"),
			func(_ string, _ *definition.Spec, ent *integration.Entity) bool {
				return !asserter.EntityMetricIs(ent, "createdKind", "deployment")
			},
			exclude.Metrics("createdAt", "createdBy", "createdKind", "deploymentName"),
		),

		// Exclude metrics known to be missing for pods that are pending.
		exclude.Exclude(
			exclude.Groups("pod", "container"),
			func(group string, _ *definition.Spec, ent *integration.Entity) bool {
				return !asserter.EntityMetricIs(ent, "status", "running")
			},
			exclude.Metrics(notRunningMetrics...),
		),

		// Reason and message are only present where a pod/container is pending or terminated
		exclude.Exclude(
			exclude.Groups("pod", "container"),
			func(_ string, _ *definition.Spec, ent *integration.Entity) bool {
				return !asserter.EntityMetricIs(ent, "status", "pending") &&
					!asserter.EntityMetricIs(ent, "status", "terminated")
			},
			exclude.Metrics("reason", "message"),
		),

		// Fair scheduler metrics are not present sometimes.
		// TODO: Investigate further why.
		exclude.Exclude(
			func(_ string, spec *definition.Spec, _ *integration.Entity) bool {
				return strings.HasPrefix(spec.Name, "containerCpuCfs")
			},
		),

		// Exclude PVC metrics for volumes that are not named "pv"
		exclude.Exclude(
			exclude.Groups("volume"),
			func(_ string, spec *definition.Spec, e *integration.Entity) bool {
				return e.Metadata.Name != "pv" && strings.HasPrefix(spec.Name, "pvc")
			},
		),
	}
}
