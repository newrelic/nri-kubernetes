package grouper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

func TestAddServiceSpecSelectorToGroup(t *testing.T) {
	svc := &v1.Service{
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"l1": "v1",
				"l2": "v2",
			},
		},
	}
	svc.Namespace = "kube-system"
	svc.Name = "kube-state-metrics"

	k8sClient := fake.NewSimpleClientset(svc)

	serviceDiscoverer, _ := discovery.NewServicesLister(k8sClient)

	grouper := &grouper{
		Config: Config{
			ServicesLister: serviceDiscoverer,
		},
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
