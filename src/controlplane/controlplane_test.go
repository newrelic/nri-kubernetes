package controlplane_test

// This file holds the integration tests for the KSM package.

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/testutil"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	excludeCM   = []string{"workqueueAddsDelta", "workqueueDepth", "workqueueDepth", "workqueueRetriesDelta"}
	excludeETCD = []string{"processFdsUtilization"}
	excludeS    = []string{"restClientRequestsDelta", "restClientRequestsRate", "schedulerScheduleAttemptsDelta", "schedulerScheduleAttemptsRate", "schedulerSchedulingDurationSeconds"}
	excludeAS   = []string{"apiserverRequestsDelta", "apiserverRequestsRate", "restClientRequestsDelta", "restClientRequestsRate", "etcdObjectCounts"}
)

const masterNodeName = "masterNode"

func Test_Scraper_Autodiscover_all_cp_components(t *testing.T) {
	t.Parallel()

	// Create an asserter with the settings that are shared for all test scenarios.
	controlPlainSpecs := definition.SpecGroups{}
	controlPlainSpecs["controller-manager"] = metric.ControllerManagerSpecs["controller-manager"]
	controlPlainSpecs["etcd"] = metric.EtcdSpecs["etcd"]
	controlPlainSpecs["scheduler"] = metric.SchedulerSpecs["scheduler"]
	controlPlainSpecs["api-server"] = metric.APIServerSpecs["api-server"]

	asserter := testutil.NewAsserter().
		Using(controlPlainSpecs).
		Excluding(
			testutil.ExcludeMetrics("controller-manager", excludeCM...),
			testutil.ExcludeMetrics("etcd", excludeETCD...),
			testutil.ExcludeMetrics("scheduler", excludeS...),
			testutil.ExcludeMetrics("api-server", excludeAS...),
		)

	for _, version := range testutil.AllVersions() {
		t.Run(fmt.Sprintf("for_version_%s", version), func(t *testing.T) {
			t.Parallel()

			testServer, err := version.Server()
			if err != nil {
				t.Fatalf("Cannot create fake KSM server: %v", err)
			}

			fakeK8s := fake.NewSimpleClientset(testutil.K8sEverything()...)

			i := testutil.NewIntegration(t)

			discoveryConfig := testConfigAutodiscovery(testServer)

			createControlPlainPods(t, fakeK8s, discoveryConfig, masterNodeName)

			testConfig := testConfig(discoveryConfig, masterNodeName)

			scraper, err := controlplane.NewScraper(
				&testConfig,
				controlplane.Providers{K8s: fakeK8s},
			)

			if err = scraper.Run(i); err != nil {
				t.Fatalf("running scraper: %v", err)
			}

			// Call the asserter for the entities of this particular sub-test.
			asserter.On(i.Entities).Assert(t)
		})
	}
}

func Test_Scraper_Autodiscover_cp_component_after_start(t *testing.T) {
	t.Parallel()

	asserter := testutil.NewAsserter().
		Using(metric.SchedulerSpecs).
		Excluding(testutil.ExcludeMetrics("scheduler", excludeS...))

	testServer, err := testutil.LatestVersion().Server()
	if err != nil {
		t.Fatalf("Cannot create fake KSM server: %v", err)
	}

	fakeK8s := fake.NewSimpleClientset(testutil.K8sEverything()...)

	i := testutil.NewIntegration(t)

	discoveryConfig := testConfigAutodiscovery(testServer)

	testConfig := testConfig(discoveryConfig, masterNodeName)

	scraper, err := controlplane.NewScraper(
		&config.Config{
			NodeName: masterNodeName,
			ControlPlane: config.ControlPlane{
				Enabled:   true,
				Scheduler: testConfig.ControlPlane.Scheduler,
			},
		},
		controlplane.Providers{
			K8s: fakeK8s,
		},
	)

	// create a scheduler pod on different node
	createControlPlainPod(t, fakeK8s, controlplane.Scheduler, discoveryConfig[controlplane.Scheduler], "masterNode2")

	if err = scraper.Run(i); err != nil {
		t.Fatalf("running scraper shouldn't fail if autodiscovery doesn't found a matching pod: %v", err)
	}

	// There is no scheduler on the same node.
	if len(i.Entities) != 0 {
		t.Fatalf("No entities should be collected before creating the pods.")
	}

	createControlPlainPod(t, fakeK8s, controlplane.Scheduler, discoveryConfig[controlplane.Scheduler], masterNodeName)

	if err = scraper.Run(i); err != nil {
		t.Fatalf("running scraper: %v", err)
	}
	// Call the asserter for the entities of this particular sub-test.
	asserter.On(i.Entities).Assert(t)
}

func Test_Scraper_external_endpoint(t *testing.T) {
	t.Parallel()

	asserter := testutil.NewAsserter().
		Using(metric.SchedulerSpecs).
		Excluding(testutil.ExcludeMetrics("scheduler", excludeS...))

	testServer, err := testutil.LatestVersion().Server()
	if err != nil {
		t.Fatalf("Cannot create fake KSM server: %v", err)
	}

	fakeK8s := fake.NewSimpleClientset(testutil.K8sEverything()...)

	i := testutil.NewIntegration(t)

	scraper, err := controlplane.NewScraper(
		&config.Config{
			NodeName: masterNodeName,
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
		NodeName: nodeName,
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

func createControlPlainPods(
	t *testing.T,
	client *fake.Clientset,
	autodiscovery map[controlplane.ComponentName]config.AutodiscoverControlPlane,
	nodeName string,
) {
	t.Helper()

	for componentName, autodiscovery := range autodiscovery {
		createControlPlainPod(t, client, componentName, autodiscovery, nodeName)
	}
	time.Sleep(time.Second)
}

func createControlPlainPod(
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
			Name:      string(componentName) + fmt.Sprintf("%d", rand.Int()), // add rand to allow create same component multiple times
			Namespace: autodiscovery.Namespace,
			Labels:    labelsSet,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
	}
	if _, err := client.CoreV1().Pods(autodiscovery.Namespace).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fail()
	}

	time.Sleep(time.Second)
}
