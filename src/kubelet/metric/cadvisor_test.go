package metric

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"os"

	"strings"

	"github.com/newrelic/nri-kubernetes/src/data"
	"github.com/newrelic/nri-kubernetes/src/kubelet/metric/testdata"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
	"github.com/stretchr/testify/assert"
)

var cadvisorQueries = []prometheus.Query{
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

func readerToHandler(r io.Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		io.Copy(w, r) // nolint: errcheck
	}
}

func TestCadvisorFetchFunc(t *testing.T) {
	f, err := os.Open("testdata/kubelet_metrics_cadvisor_payload_plain.txt")
	if err != nil {
		panic(err)
	}

	defer f.Close()

	c := testClient{
		handler: readerToHandler(f),
	}

	g, err := CadvisorFetchFunc(&c, cadvisorQueries)()

	assert.NoError(t, err)
	assert.Equal(t, testdata.ExpectedCadvisorRawData, g)
}

func TestCadvisorFetchFunc_MissingLabels(t *testing.T) {
	f := strings.NewReader(`
container_memory_usage_bytes{id="/kubepods/podf89b6c09-11a3-11e8-a084-080027352a02/3328c17bfd22f1a82fcdf8707c2f8f040c462e548c24780079bba95d276d93e1",image="gcr.io/google_containers/addon-resizer@sha256:e77acf80697a70386c04ae3ab494a7b13917cb30de2326dcf1a10a5118eddabe",name="k8s_addon-resizer_kube-state-metrics-57f4659995-6n2qq_kube-system_f89b6c09-11a3-11e8-a084-080027352a02_17",namespace="kube-system",pod_name="kube-state-metrics-57f4659995-6n2qq"} 1.7788928e+07
container_memory_usage_bytes{container_name="dnsmasq",id="/kubepods/burstable/podbb3b914c-11a3-11e8-a084-080027352a02/81de1e9aba1c051a2f9780a5db594a899c9e4e76613d4c95da4561cc48e8658f",image="sha256:459944ce8cc4f08ebade5c05bb884e4da053d73e61ec6afe82a0b1687317254c",name="k8s_dnsmasq_kube-dns-54cccfbdf8-dznm7_kube-system_bb3b914c-11a3-11e8-a084-080027352a02_13",pod_name="kube-dns-54cccfbdf8-dznm7"} 1.4655488e+07
container_memory_usage_bytes{container_name="grafana",id="/kubepods/besteffort/podbb233a6b-11a3-11e8-a084-080027352a02/7f092105225a729f4917aa6950b5b90236c720fc411eee80ba9f7ca0f639525f",image="k8s.gcr.io/heapster-grafana-amd64@sha256:4a472eb4df03f4f557d80e7c6b903d9c8fe31493108b99fbd6da6540b5448d70",name="k8s_grafana_influxdb-grafana-rsmwp_kube-system_bb233a6b-11a3-11e8-a084-080027352a02_17",namespace="kube-system"} 2.5956352e+07
container_memory_usage_bytes{container_name="heapster",image="k8s.gcr.io/heapster-amd64@sha256:da3288b0fe2312c621c2a6d08f24ccc56183156ec70767987501287db4927b9d",name="k8s_heapster_heapster-5mz5f_kube-system_bb142c1b-11a3-11e8-a084-080027352a02_15",namespace="kube-system",pod_name="heapster-5mz5f"} 3.0572544e+07
container_memory_usage_bytes{container_name="influxdb",id="/kubepods/besteffort/podbb233a6b-11a3-11e8-a084-080027352a02/fd0ca055e308e5d11b0c8fbf273b733d1166aa2823bf7fd724a6b70c72959774",name="k8s_influxdb_influxdb-grafana-rsmwp_kube-system_bb233a6b-11a3-11e8-a084-080027352a02_17",namespace="kube-system",pod_name="influxdb-grafana-rsmwp"} 7.4510336e+07
`)

	c := testClient{
		handler: readerToHandler(f),
	}

	_, err := CadvisorFetchFunc(&c, cadvisorQueries)()
	assert.Error(t, err)

	expectedErrs := []error{
		errors.New("container name not found in cadvisor metrics"),
		errors.New("namespace not found in cadvisor metrics"),
		errors.New("pod name not found in cadvisor metrics"),
		errors.New("container id not found in cadvisor metrics"),
		errors.New("container image not found in cadvisor metrics"),
	}

	assert.ElementsMatch(t, expectedErrs, err.(data.ErrorGroup).Errors)
}
