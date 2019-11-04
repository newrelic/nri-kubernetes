package main

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/src/apiserver"
	"github.com/newrelic/nri-kubernetes/src/controlplane"
	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/newrelic/nri-kubernetes/src/storage"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var logger = logrus.StandardLogger()

func TestControlPlaneJobs(t *testing.T) {
	nodeName := "ip-10.0.2.15"
	nodeIP := "10.0.2.15"
	tmpDir, err := ioutil.TempDir("", "test_discover")
	assert.NoError(t, err)
	// Setup the data returned when querying the kubelet for the pods
	// running on the node.
	rawGroups := definition.RawGroups{
		"pod": make(map[string]definition.RawMetrics),
	}

	components := controlplane.BuildComponentList()

	for _, com := range components {
		var labelKey, labelValue string
		for labelKey, labelValue = range com.Labels {
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
	// Setup storage
	store := storage.NewJSONDiskStorage(tmpDir)

	// Setup the fake api server with the labels belonging to a master node.
	apiServerClient := apiserver.TestAPIServer{
		Mem: map[string]*apiserver.NodeInfo{
			nodeName: {
				NodeName: nodeName,
				Labels: map[string]string{
					"kubernetes.io/role": "master",
				},
			},
		},
	}

	// When getting the control plane jobs for this node
	cpJobs, err := controlPlaneJobs(
		logger,
		apiServerClient,
		nodeName,
		time.Duration(0),
		time.Duration(0),
		store,
		nodeIP,
		podsFetcher,
		nil,
		"test",
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
