package grouper

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/src/data"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/kubelet/client"
	"github.com/newrelic/nri-kubernetes/v3/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v3/src/kubelet/metric/testdata"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"

	"github.com/google/go-cmp/cmp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type testClient struct {
	handler http.HandlerFunc
}

func (c *testClient) GetURI(uri url.URL) (*http.Response, error) {
	req := httptest.NewRequest(http.MethodGet, uri.String(), nil)
	return c.Do(req)
}

func (c *testClient) Get(path string) (*http.Response, error) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	return c.Do(req)
}

func (c *testClient) Do(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()

	c.handler(w, req)

	return w.Result(), nil
}

func rawGroupsHandlerFunc(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case metric.KubeletPodsPath:
		f, err := os.Open("../metric/testdata/kubelet_pods_payload.json") // TODO move fetch and testdata to just kubelet package.
		if err != nil {
			panic(err)
		}

		defer f.Close() // nolint: errcheck

		io.Copy(w, f) // nolint: errcheck
	case metric.StatsSummaryPath:
		f, err := os.Open("../metric/testdata/kubelet_stats_summary_payload.json") // TODO move fetch and testdata to just kubelet package.
		if err != nil {
			panic(err)
		}

		defer f.Close() // nolint: errcheck

		io.Copy(w, f) // nolint: errcheck
	case metric.KubeletCAdvisorMetricsPath:
		f, err := os.Open("../metric/testdata/k8s_v1_15_kubelet_metrics_cadvisor_payload_plain.txt")
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

	k8sClient := fake.NewSimpleClientset(getNode())
	nodeGetter, _ := discovery.NewNodeLister(k8sClient)

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

	podsFetcher := metric.NewBasicPodsFetcher(
		log.StandardLogger(),
		&c,
	)

	kubeletClient, err := client.New(client.StaticConnector(&c, url.URL{}), client.WithMaxRetries(3))
	require.NoError(t, err)

	kubeletGrouper, err := New(
		Config{
			NodeGetter: nodeGetter,
			Client:     kubeletClient,
			Fetchers: []data.FetchFunc{
				podsFetcher.DoPodsFetch,
				metric.CadvisorFetchFunc(kubeletClient.MetricFamiliesGetFunc(metric.KubeletCAdvisorMetricsPath), queries),
			},
			DefaultNetworkInterface: "eth0",
		},
	)
	assert.Nil(t, err)

	r, errGroup := kubeletGrouper.Group(nil)

	assert.Nil(t, errGroup)

	if diff := cmp.Diff(testdata.ExpectedGroupData, r); diff != "" {
		t.Errorf("unexpected difference: %s", diff)
	}
}

func TestCountRunningPods(t *testing.T) {
	t.Parallel()
	g := &grouper{}

	rawGroups := definition.RawGroups{
		"pod": {
			"pod1": definition.RawMetrics{"status": "Running"},
			"pod2": definition.RawMetrics{"status": "Pending"},
			"pod3": definition.RawMetrics{"status": "Running"},
			"pod4": definition.RawMetrics{"status": "Failed"},
			"pod5": definition.RawMetrics{"status": "Running"},
		},
	}

	count := g.countRunningPods(rawGroups)
	require.Equal(t, 3, count, "Should count only pods with status 'Running'")
}

func getNode() *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "minikube",
			Labels: map[string]string{
				"kubernetes.io/arch":                    "amd64",
				"kubernetes.io/hostname":                "minikube",
				"kubernetes.io/os":                      "linux",
				"node-role.kubernetes.io/control-plane": "",
			},
		},
		Spec: v1.NodeSpec{
			Unschedulable: false,
		},
		Status: v1.NodeStatus{
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
			NodeInfo: v1.NodeSystemInfo{
				KubeletVersion: "v1.22.1",
			},
		},
	}
}
