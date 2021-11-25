package controlplane_test

// This file holds the integration tests for the KSM package.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/testutil"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	excludeCM   = []string{"workqueueAddsDelta", "workqueueDepth", "workqueueDepth", "workqueueRetriesDelta"}
	excludeETCD = []string{"processFdsUtilization"}
	excludeS    = []string{"restClientRequestsDelta", "restClientRequestsRate", "schedulerScheduleAttemptsDelta", "schedulerScheduleAttemptsRate", "schedulerSchedulingDurationSeconds"}
	excludeAS   = []string{"apiserverRequestsDelta", "apiserverRequestsRate", "restClientRequestsDelta", "restClientRequestsRate", "etcdObjectCounts"}
)

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

			scraper, err := controlplane.NewScraper(&config.Mock{
				ControlPlane: config.ControlPlane{
					ControllerManager: config.ControllerManager{
						ControllerManagerEndpointURL: testServer.ControlPlaneEndpoint(string(controlplane.ControllerManager)),
					},
					ETCD: config.ETCD{
						EtcdEndpointURL: testServer.ControlPlaneEndpoint(string(controlplane.Etcd)),
					},
					APIServer: config.APIServer{
						APIServerEndpointURL: testServer.ControlPlaneEndpoint(string(controlplane.APIServer)),
					},
					Scheduler: config.Scheduler{
						SchedulerEndpointURL: testServer.ControlPlaneEndpoint(string(controlplane.Scheduler)),
					},
				},
				ClusterName: t.Name(),
			}, controlplane.Providers{
				K8s: fakeK8s,
			})

			createControlPlainPods(t, fakeK8s, standardTestComponents())

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

	scraper, err := controlplane.NewScraper(&config.Mock{
		ControlPlane: config.ControlPlane{
			Scheduler: config.Scheduler{
				SchedulerEndpointURL: testServer.ControlPlaneEndpoint(string(controlplane.Scheduler)),
			},
		},
		ClusterName: t.Name(),
	}, controlplane.Providers{
		K8s: fakeK8s,
	})

	if err = scraper.Run(i); err != nil {
		t.Fatalf("running scraper: %v", err)
	}

	if len(i.Entities) != 0 {
		t.Fatalf("No entities should be collected before creating the pods.")
	}

	createControlPlainPods(t, fakeK8s, standardTestComponents())

	if err = scraper.Run(i); err != nil {
		t.Fatalf("running scraper: %v", err)
	}
	// Call the asserter for the entities of this particular sub-test.
	asserter.On(i.Entities).Assert(t)
}

type testComponent struct {
	Name      controlplane.ComponentName
	NameSpace string
	Labels    map[string]string
}

func standardTestComponents() []testComponent {
	return []testComponent{
		{
			Name:      controlplane.Scheduler,
			NameSpace: "kube-system",
			Labels:    map[string]string{"k8s-app": "kube-scheduler"},
		},
		{
			Name:      controlplane.Etcd,
			NameSpace: "kube-system",
			Labels:    map[string]string{"k8s-app": "etcd-manager-main"},
		},
		{
			Name:      controlplane.ControllerManager,
			NameSpace: "kube-system",
			Labels:    map[string]string{"k8s-app": "kube-controller-manager"},
		},
		{
			Name:      controlplane.APIServer,
			NameSpace: "kube-system",
			Labels:    map[string]string{"k8s-app": "kube-apiserver"},
		},
	}
}

func createControlPlainPods(t *testing.T, client *fake.Clientset, testComponents []testComponent) {
	t.Helper()

	for _, component := range testComponents {

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      string(component.Name),
				Namespace: component.NameSpace,
				Labels:    component.Labels,
			},
		}
		if _, err := client.CoreV1().Pods(component.NameSpace).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
			t.Fail()
		}
	}
	time.Sleep(time.Second)
}
