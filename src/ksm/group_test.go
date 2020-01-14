package ksm

import (
	"testing"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestAddServiceSpecSelectorToGroup(t *testing.T) {
	k8sClient := new(client.MockedKubernetes)
	serviceList := &v1.ServiceList{
		Items: []v1.Service{
			{
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"l1": "v1",
						"l2": "v2",
					},
				},
			},
		},
	}
	serviceList.Items[0].Namespace = "kube-system"
	serviceList.Items[0].Name = "kube-state-metrics"
	k8sClient.On("ListServices").Return(serviceList, nil)

	grouper := &ksmGrouper{
		k8sClient: k8sClient,
	}

	serviceGroup := map[string]definition.RawMetrics{
		"kube-system_kube-state-metrics": make(definition.RawMetrics),
	}
	err := grouper.addServiceSpecSelectorToGroup(serviceGroup)
	require.NoError(t, err)
	expected := prometheus.Labels{"selector_l1": "v1", "selector_l2": "v2"}
	actual := serviceGroup["kube-system_kube-state-metrics"]["apiserver_kube_service_spec_selectors"].(prometheus.Metric).Labels
	assert.Equal(t, expected["selector_l1"], actual["selector_l1"])
	assert.Equal(t, expected["selector_l2"], actual["selector_l2"])
}
