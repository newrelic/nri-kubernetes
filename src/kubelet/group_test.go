package kubelet

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/newrelic/nri-kubernetes/v2/src/apiserver"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric/testdata"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

type testClient struct {
	handler http.HandlerFunc
}

func (c *testClient) Get(path string) (*http.Response, error) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()

	c.handler(w, req)

	return w.Result(), nil
}

func (c *testClient) NodeIP() string {
	// nothing to do
	return ""
}

func rawGroupsHandlerFunc(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case metric.KubeletPodsPath:
		f, err := os.Open("metric/testdata/kubelet_pods_payload.json") // TODO move fetch and testdata to just kubelet package.
		if err != nil {
			panic(err)
		}

		defer f.Close() // nolint: errcheck

		io.Copy(w, f) // nolint: errcheck
	case metric.StatsSummaryPath:
		f, err := os.Open("metric/testdata/kubelet_stats_summary_payload.json") // TODO move fetch and testdata to just kubelet package.
		if err != nil {
			panic(err)
		}

		defer f.Close() // nolint: errcheck

		io.Copy(w, f) // nolint: errcheck
	case metric.KubeletCAdvisorMetricsPath:
		f, err := os.Open("metric/testdata/k8s_v1_15_kubelet_metrics_cadvisor_payload_plain.txt")
		if err != nil {
			panic(err)
		}
		defer f.Close() // nolint: errcheck
		w.Header().Set("Content-Type", "text/plain")
		io.Copy(w, f) // nolint: errcheck
	}
}

func TestGroup(t *testing.T) {
	c := testClient{
		handler: rawGroupsHandlerFunc,
	}
	a := apiserver.TestAPIServer{Mem: map[string]*apiserver.NodeInfo{
		"minikube": {
			NodeName: "minikube",
			Labels: map[string]string{
				"kubernetes.io/arch":             "amd64",
				"kubernetes.io/hostname":         "minikube",
				"kubernetes.io/os":               "linux",
				"node-role.kubernetes.io/master": "",
			},
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:              *resource.NewQuantity(2, resource.DecimalSI),
				v1.ResourcePods:             *resource.NewQuantity(110, resource.DecimalSI),
				v1.ResourceEphemeralStorage: *resource.NewQuantity(18211580000, resource.BinarySI),
				v1.ResourceMemory:           *resource.NewQuantity(2033280000, resource.BinarySI),
			},
			Capacity: v1.ResourceList{
				v1.ResourceCPU:              *resource.NewQuantity(2, resource.DecimalSI),
				v1.ResourcePods:             *resource.NewQuantity(110, resource.DecimalSI),
				v1.ResourceEphemeralStorage: *resource.NewQuantity(18211586048, resource.BinarySI),
				v1.ResourceMemory:           *resource.NewQuantity(2033283072, resource.BinarySI),
			},
			Conditions: []v1.NodeCondition{
				{
					Type:   "TrueCondition",
					Status: v1.ConditionTrue,
				},
				{
					Type:   "FalseCondition",
					Status: v1.ConditionFalse,
				},
				{
					Type:   "UnknownCondition",
					Status: v1.ConditionUnknown,
				},
				{
					Type:   "DuplicatedCondition",
					Status: v1.ConditionTrue,
				},
				{
					Type:   "DuplicatedCondition",
					Status: v1.ConditionFalse,
				},
			},
			Unschedulable:  false,
			KubeletVersion: "v1.22.1",
		},
	}}
	queries := []prometheus.Query{
		{
			MetricName: "container_memory_usage_bytes",
			Labels: prometheus.QueryLabels{
				Operator: prometheus.QueryOpNor,
				Labels: prometheus.Labels{
					"container_name": "",
				},
			},
		},
	}

	podsFetcher := metric.NewPodsFetcher(
		logrus.StandardLogger(),
		&c,
	)
	grouper := NewGrouper(
		&c,
		logrus.StandardLogger(),
		a,
		"eth0",
		podsFetcher.FetchFuncWithCache(),
		metric.CadvisorFetchFunc(&c, queries),
	)
	r, errGroup := grouper.Group(nil)

	assert.Nil(t, errGroup)
	assert.Equal(t, testdata.ExpectedGroupData, r)

}
