package controlplane_test

// This file holds the integration tests for the ControlPlane package.

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/testutil"
	"github.com/newrelic/nri-kubernetes/v3/internal/testutil/asserter"
	"github.com/newrelic/nri-kubernetes/v3/internal/testutil/asserter/exclude"
	"github.com/newrelic/nri-kubernetes/v3/src/controlplane"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/metric"
)

var (
	excludeCM = []string{"leaderElectionMasterStatus"}
	excludeS  = []string{
		"leaderElectionMasterStatus",
		"schedulerSchedulingDurationSeconds",
		"schedulerPreemptionAttemptsDelta",
		"schedulerPodPreemptionVictims",
	}
)

const (
	masterNodeName = "masterNode"
	clusterName    = "testClusterName"
)

func Test_Scraper_Autodiscover_all_cp_components(t *testing.T) {
	t.Parallel()

	// Create an asserter with the settings that are shared for all test scenarios.
	controlPlaneSpecs := definition.SpecGroups{}
	controlPlaneSpecs["controller-manager"] = metric.ControllerManagerSpecs["controller-manager"]
	controlPlaneSpecs["etcd"] = metric.EtcdSpecs["etcd"]
	controlPlaneSpecs["scheduler"] = metric.SchedulerSpecs["scheduler"]
	controlPlaneSpecs["api-server"] = metric.APIServerSpecs["api-server"]

	asserter := asserter.New().
		Silently().
		Using(controlPlaneSpecs).
		Excluding(
			ExcludeRenamedMetricsBasedOnLabels,
			exclude.Exclude(
				exclude.Groups("controller-manager"),
				exclude.Metrics(excludeCM...),
			),
			exclude.Exclude(
				exclude.Groups("scheduler"),
				exclude.Metrics(excludeS...),
			),
		)

	for _, v := range testutil.AllVersions() {
		version := v
		t.Run(fmt.Sprintf("for_version_%s", version), func(t *testing.T) {
			t.Parallel()

			testServer, err := version.Server()
			if err != nil {
				t.Fatalf("Cannot create fake KSM server: %v", err)
			}

			k8sData, err := version.K8s()
			if err != nil {
				t.Fatalf("error instantiating fake k8s objects: %v", err)
			}

			fakeK8s := fake.NewSimpleClientset(k8sData.Everything()...)

			i := testutil.NewIntegration(t)

			discoveryConfig := testConfigAutodiscovery(testServer)

			createControlPlanePods(t, fakeK8s, discoveryConfig, masterNodeName)

			testConfig := testConfig(discoveryConfig, masterNodeName)

			scraper, err := controlplane.NewScraper(
				&testConfig,
				controlplane.Providers{K8s: fakeK8s},
			)
			if err != nil {
				t.Fatalf("error building scraper: %v", err)
			}

			if err = scraper.Run(i); err != nil {
				t.Fatalf("running scraper: %v", err)
			}

			// Include specific exclusions that depend on version.
			versionAsserter := asserter

			// apiserverStorageObjects replaces etcObjectCounts in k8s versions above 1.23
			versionAsserter = versionAsserter.Excluding(exclude.Metrics("etcdObjectCounts"))

			// Call the asserter for the entities of this particular sub-test.
			versionAsserter.On(i.Entities).Assert(t)
		})
	}
}

func Test_Scraper_Autodiscover_cp_component_after_start(t *testing.T) {
	t.Parallel()

	asserter := asserter.New().
		Silently().
		Using(metric.SchedulerSpecs).
		Excluding(
			ExcludeRenamedMetricsBasedOnLabels,
			exclude.Exclude(
				exclude.Groups("scheduler"),
				exclude.Metrics(excludeS...),
			),
		)

	testServer, err := testutil.LatestVersion().Server()
	if err != nil {
		t.Fatalf("Cannot create fake KSM server: %v", err)
	}

	k8sData, err := testutil.LatestVersion().K8s()
	if err != nil {
		t.Fatalf("error instantiating fake k8s objects: %v", err)
	}

	fakeK8s := fake.NewSimpleClientset(k8sData.Everything()...)

	i := testutil.NewIntegration(t)

	discoveryConfig := testConfigAutodiscovery(testServer)

	testConfig := testConfig(discoveryConfig, masterNodeName)

	scraper, err := controlplane.NewScraper(
		&config.Config{
			ClusterName: clusterName,
			NodeName:    masterNodeName,
			ControlPlane: config.ControlPlane{
				Enabled:   true,
				Scheduler: testConfig.ControlPlane.Scheduler,
			},
		},
		controlplane.Providers{
			K8s: fakeK8s,
		},
	)
	if err != nil {
		t.Fatalf("error building scraper: %v", err)
	}

	// create a scheduler pod on different node
	createControlPlanePod(t, fakeK8s, controlplane.Scheduler, discoveryConfig[controlplane.Scheduler], "masterNode2")

	if err = scraper.Run(i); err != nil {
		t.Fatalf("running scraper shouldn't fail if autodiscovery doesn't found a matching pod: %v", err)
	}

	// There is no scheduler on the same node.
	if len(i.Entities) != 0 {
		t.Fatalf("No entities should be collected before creating the pods.")
	}

	createControlPlanePod(t, fakeK8s, controlplane.Scheduler, discoveryConfig[controlplane.Scheduler], masterNodeName)

	if err = scraper.Run(i); err != nil {
		t.Fatalf("running scraper: %v", err)
	}
	// Call the asserter for the entities of this particular sub-test.
	asserter.On(i.Entities).Assert(t)
}

func Test_Scraper_external_endpoint(t *testing.T) {
	t.Parallel()

	asserter := asserter.New().
		Silently().
		Using(metric.SchedulerSpecs).
		Excluding(
			ExcludeRenamedMetricsBasedOnLabels,
			exclude.Exclude(
				exclude.Groups("scheduler"),
				exclude.Metrics(excludeS...),
			),
		)

	testServer, err := testutil.LatestVersion().Server()
	if err != nil {
		t.Fatalf("Cannot create fake KSM server: %v", err)
	}

	k8sData, err := testutil.LatestVersion().K8s()
	if err != nil {
		t.Fatalf("error instantiating fake k8s objects: %v", err)
	}

	fakeK8s := fake.NewSimpleClientset(k8sData.Everything()...)

	i := testutil.NewIntegration(t)

	scraper, err := controlplane.NewScraper(
		&config.Config{
			ClusterName: clusterName,
			NodeName:    masterNodeName,
			ControlPlane: config.ControlPlane{
				Enabled: true,
				Scheduler: config.ControlPlaneComponent{
					Enabled: true,
					StaticEndpoint: &config.Endpoint{
						URL: testServer.ControlPlaneEndpoint(string(controlplane.Scheduler)),
					},
				},
			},
		},
		controlplane.Providers{
			K8s: fakeK8s,
		},
	)
	if err != nil {
		t.Fatalf("error building scraper: %v", err)
	}

	if err = scraper.Run(i); err != nil {
		t.Fatalf("running scraper: %v", err)
	}
	// Call the asserter for the entities of this particular sub-test.
	asserter.On(i.Entities).Assert(t)

	testServer.Close()

	if err = scraper.Run(i); err == nil {
		t.Fatalf("scraper should fail if static endpoint cannot be scraped")
	}
}

func testConfigAutodiscovery(server *testutil.Server) map[controlplane.ComponentName]config.AutodiscoverControlPlane {
	const defaultNamespace = "kube-system"

	return map[controlplane.ComponentName]config.AutodiscoverControlPlane{
		controlplane.Etcd: {
			Namespace: defaultNamespace,
			MatchNode: true,
			Selector:  "k8s-app=etcd-manager-main",
			Endpoints: []config.Endpoint{
				{URL: server.ControlPlaneEndpoint(string(controlplane.Etcd))},
			},
		},
		controlplane.APIServer: {
			Namespace: defaultNamespace,
			MatchNode: true,
			Selector:  "k8s-app=kube-apiserver",
			Endpoints: []config.Endpoint{
				{URL: server.ControlPlaneEndpoint(string(controlplane.APIServer))},
			},
		},
		controlplane.Scheduler: {
			Namespace: defaultNamespace,
			MatchNode: true,
			Selector:  "k8s-app=kube-scheduler",
			Endpoints: []config.Endpoint{
				{URL: server.ControlPlaneEndpoint(string(controlplane.Scheduler))},
			},
		},
		controlplane.ControllerManager: {
			Namespace: defaultNamespace,
			MatchNode: true,
			Selector:  "k8s-app=kube-controller-manager",
			Endpoints: []config.Endpoint{
				{URL: server.ControlPlaneEndpoint(string(controlplane.ControllerManager))},
			},
		},
	}
}

func testConfig(
	autodiscovery map[controlplane.ComponentName]config.AutodiscoverControlPlane,
	nodeName string,
) config.Config {
	return config.Config{
		ClusterName: clusterName,
		NodeName:    nodeName,
		ControlPlane: config.ControlPlane{
			Enabled: true,
			ETCD: config.ControlPlaneComponent{
				Enabled: true,
				Autodiscover: []config.AutodiscoverControlPlane{
					autodiscovery[controlplane.Etcd],
				},
			},
			APIServer: config.ControlPlaneComponent{
				Enabled: true,
				Autodiscover: []config.AutodiscoverControlPlane{
					autodiscovery[controlplane.APIServer],
				},
			},
			ControllerManager: config.ControlPlaneComponent{
				Enabled: true,
				Autodiscover: []config.AutodiscoverControlPlane{
					autodiscovery[controlplane.ControllerManager],
				},
			},
			Scheduler: config.ControlPlaneComponent{
				Enabled: true,
				Autodiscover: []config.AutodiscoverControlPlane{
					autodiscovery[controlplane.Scheduler],
				},
			},
		},
	}
}

func createControlPlanePods(
	t *testing.T,
	client *fake.Clientset,
	autodiscovery map[controlplane.ComponentName]config.AutodiscoverControlPlane,
	nodeName string,
) {
	t.Helper()

	for componentName, autodiscovery := range autodiscovery {
		createControlPlanePod(t, client, componentName, autodiscovery, nodeName)
	}

	time.Sleep(time.Second)
}

func createControlPlanePod(
	t *testing.T,
	client *fake.Clientset,
	componentName controlplane.ComponentName,
	autodiscovery config.AutodiscoverControlPlane,
	nodeName string,
) {
	t.Helper()
	labelsSet, _ := labels.ConvertSelectorToLabelsMap(autodiscovery.Selector)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(componentName) + fmt.Sprintf("%d", rand.Int()), // nosemgrep // add rand to allow create same component multiple times.
			Namespace: autodiscovery.Namespace,
			Labels:    labelsSet,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
	}
	if _, err := client.CoreV1().Pods(autodiscovery.Namespace).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatalf("error creating pods in fake client: %v", err)
	}

	time.Sleep(time.Second)
}

// ExcludeRenamedMetricsBasedOnLabels check if any of the metrics exist with a different name starting
// with 'metricName_'. This covers the case where metric names are changed based on labels
// ie: metric 'workqueueAddsDelta' can be found as "workqueueAddsDelta_name_garbage_collector_attempt_to_delete"
// more info about this in prometheus.fetchedValuesFromRawMetrics() function description.
func ExcludeRenamedMetricsBasedOnLabels(_ string, spec *definition.Spec, ent *integration.Entity) bool {
	for _, metricSet := range ent.Metrics {
		for k := range metricSet.Metrics {
			if strings.HasPrefix(k, spec.Name+"_") {
				return true
			}
		}
	}
	return false
}
