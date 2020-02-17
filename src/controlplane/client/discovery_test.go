package client

import (
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/src/controlplane"
	"github.com/newrelic/nri-kubernetes/src/data"
	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var logger = logrus.StandardLogger()

// Testing Discover() method
func TestDiscover(t *testing.T) {

	component := controlplane.BuildComponentList()[0]
	podName := "scheduler"

	var testCases = []struct {
		name                     string
		assertIsComponentRunning func(assert.TestingT, bool, ...interface{}) bool
		assertPodName            func(string)
		podsFetcher              data.FetchFunc
	}{
		{
			name:                     "component is not running on node: missing tier",
			assertIsComponentRunning: assert.False,
			assertPodName: func(p string) {
				assert.Equal(t, "", p)
			},
			podsFetcher: func() (definition.RawGroups, error) {
				return definition.RawGroups{
					podEntityType: map[string]definition.RawMetrics{
						"kube-system_kube-scheduler-minikube": {
							"namespace": "kube-system",
							"podName":   podName,
							"nodeName":  "minikube",
							"nodeIP":    "10.0.2.15",
							"startTime": time.Now(),
							"labels": map[string]string{
								"component": "kube-scheduler",
							},
						},
					},
				}, nil
			},
		},
		{
			name:                     "component is running on node: using tier and component labels",
			assertIsComponentRunning: assert.True,
			assertPodName: func(p string) {
				assert.Equal(t, podName, p)
			},
			podsFetcher: func() (definition.RawGroups, error) {
				return definition.RawGroups{
					podEntityType: map[string]definition.RawMetrics{
						"kube-system_kube-scheduler-minikube": {
							"namespace": "kube-system",
							"podName":   podName,
							"nodeName":  "minikube",
							"nodeIP":    "10.0.2.15",
							"startTime": time.Now(),
							"labels": map[string]string{
								"component": "kube-scheduler",
								"tier":      "control-plane",
							},
						},
					},
				}, nil
			},
		},
		{
			name:                     "component is running on node: using k8s-app label",
			assertIsComponentRunning: assert.True,
			assertPodName: func(p string) {
				assert.Equal(t, podName, p)
			},
			podsFetcher: func() (definition.RawGroups, error) {
				return definition.RawGroups{
					podEntityType: map[string]definition.RawMetrics{
						"kube-system_kube-scheduler-minikube": {
							"namespace": "kube-system",
							"podName":   podName,
							"nodeName":  "minikube",
							"nodeIP":    "10.0.2.15",
							"startTime": time.Now(),
							"labels": map[string]string{
								"k8s-app":     "kube-scheduler",
								"extra-label": "iluvetests",
								"tier":        "control-plane",
							},
						},
					},
				}, nil
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given a client
			nodeIP := "6.7.8.9"

			// And a Discoverer implementation
			d := discoverer{
				logger:      logger,
				nodeIP:      nodeIP,
				component:   component,
				podsFetcher: testCase.podsFetcher,
			}

			// When retrieving the KSM client
			cl, err := d.Discover(0)
			cpC := cl.(*ControlPlaneComponentClient)
			// The call works correctly
			assert.Nil(t, err, "should not return error")
			testCase.assertIsComponentRunning(t, cpC.IsComponentRunningOnNode)
			assert.Equal(t, component.Endpoint, cpC.endpoint)
			testCase.assertPodName(cpC.PodName)
		})
	}
}

func TestDiscover_ShouldSetNoAuth_WhenBothAuthFalse(t *testing.T) {

	component := controlplane.BuildComponentList()[0]
	component.UseServiceAccountAuthentication = false
	component.UseMTLSAuthentication = false
	podName := "scheduler"

	var podsFetcher data.FetchFunc = func() (definition.RawGroups, error) {
		return definition.RawGroups{
			podEntityType: map[string]definition.RawMetrics{
				"kube-system_kube-scheduler-minikube": {
					"namespace": "kube-system",
					"podName":   podName,
					"nodeName":  "minikube",
					"nodeIP":    "10.0.2.15",
					"startTime": time.Now(),
					"labels": map[string]string{
						"k8s-app":     "kube-scheduler",
						"extra-label": "iluvetests",
						"tier":        "control-plane",
					},
				},
			},
		}, nil
	}

	// Given a client
	nodeIP := "6.7.8.9"

	// And a Discoverer implementation
	d := discoverer{
		logger:      logger,
		nodeIP:      nodeIP,
		component:   component,
		podsFetcher: podsFetcher,
	}

	// When retrieving the KSM client
	cl, err := d.Discover(0)

	assert.Nil(t, err)

	cpC := cl.(*ControlPlaneComponentClient)
	assert.Equal(t, none, cpC.authenticationMethod)
}

func TestDiscover_ShouldSetSAAuth_WhenUseSAAuthIsTrue(t *testing.T) {

	component := controlplane.BuildComponentList()[0]
	component.UseServiceAccountAuthentication = true
	podName := "scheduler"

	var podsFetcher data.FetchFunc = func() (definition.RawGroups, error) {
		return definition.RawGroups{
			podEntityType: map[string]definition.RawMetrics{
				"kube-system_kube-scheduler-minikube": {
					"namespace": "kube-system",
					"podName":   podName,
					"nodeName":  "minikube",
					"nodeIP":    "10.0.2.15",
					"startTime": time.Now(),
					"labels": map[string]string{
						"k8s-app":     "kube-scheduler",
						"extra-label": "iluvetests",
						"tier":        "control-plane",
					},
				},
			},
		}, nil
	}

	// Given a client
	nodeIP := "6.7.8.9"

	// And a Discoverer implementation
	d := discoverer{
		logger:      logger,
		nodeIP:      nodeIP,
		component:   component,
		podsFetcher: podsFetcher,
	}

	// When retrieving the KSM client
	cl, err := d.Discover(0)

	assert.Nil(t, err)

	cpC := cl.(*ControlPlaneComponentClient)
	assert.Equal(t, serviceAccount, cpC.authenticationMethod)
}

func TestDiscover_ShouldSetMTLSAuth_WhenUseMTLSAuthIsTrue(t *testing.T) {

	component := controlplane.BuildComponentList()[0]
	component.UseMTLSAuthentication = true
	podName := "scheduler"

	var podsFetcher data.FetchFunc = func() (definition.RawGroups, error) {
		return definition.RawGroups{
			podEntityType: map[string]definition.RawMetrics{
				"kube-system_kube-scheduler-minikube": {
					"namespace": "kube-system",
					"podName":   podName,
					"nodeName":  "minikube",
					"nodeIP":    "10.0.2.15",
					"startTime": time.Now(),
					"labels": map[string]string{
						"k8s-app":     "kube-scheduler",
						"extra-label": "iluvetests",
						"tier":        "control-plane",
					},
				},
			},
		}, nil
	}

	// Given a client
	nodeIP := "6.7.8.9"

	// And a Discoverer implementation
	d := discoverer{
		logger:      logger,
		nodeIP:      nodeIP,
		component:   component,
		podsFetcher: podsFetcher,
	}

	// When retrieving the KSM client
	cl, err := d.Discover(0)

	assert.Nil(t, err)

	cpC := cl.(*ControlPlaneComponentClient)
	assert.Equal(t, mTLS, cpC.authenticationMethod)
}
