package deprecated

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v2/src/controlplane"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
)

func TestControlPlaneJobs(t *testing.T) {
	nodeName := "ip-10.0.2.15"
	nodeIP := "10.0.2.15"
	// Setup the data returned when querying the kubelet for the pods
	// running on the node.
	rawGroups := definition.RawGroups{
		"pod": make(map[string]definition.RawMetrics),
	}

	components := controlplane.BuildComponentList()

	for _, com := range components {

		if len(com.Labels) == 0 {
			t.Fatalf("component %s has no labels associated", com.Name)
		}

		var labelKey, labelValue string
		for labelKey, labelValue = range com.Labels[0] {
			break
		}
		rawGroups["pod"][fmt.Sprintf("kube-system_%s-pod", com.Name)] = definition.RawMetrics(map[string]definition.RawValue{
			"namespace": "kube-system",
			"podName":   fmt.Sprintf("%s-pod", com.Name),
			"nodeName":  "minikube",
			"nodeIP":    nodeName,
			"startTime": time.Now(),
			"labels": map[string]string{
				labelKey: labelValue,
			},
		})
	}
	podsFetcher := func() (definition.RawGroups, error) {
		return rawGroups, nil
	}
	// Setup the fake api server with the labels belonging to a master node.

	fakeClient := fake.NewSimpleClientset(&v1.Node{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				"kubernetes.io/role": "master",
			},
		},
		Spec:   v1.NodeSpec{},
		Status: v1.NodeStatus{},
	})

	// When getting the control plane jobs for this node
	cpJobs, _ := controlPlaneJobs(
		logger,
		nodeName,
		time.Duration(0),
		nodeIP,
		podsFetcher,
		fakeClient,
		"test",
		"",
		"",
		"",
		"",
		"",
		"",
	)
	assert.Equal(t, len(components), len(cpJobs))

	// For every component there is a job with its name and its specs
	for _, com := range components {
		jobFound := false
		for _, j := range cpJobs {
			if j.Name == string(com.Name) {
				assert.Equal(t, com.Specs, j.Specs)
				assert.NotNil(t, j.Grouper)
				jobFound = true
			}
		}
		if !jobFound {
			t.Errorf("No job found for the controlplane component %s", com.Name)
		}
	}
}
