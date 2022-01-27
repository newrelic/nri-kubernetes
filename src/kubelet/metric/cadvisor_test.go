package metric

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v3/src/data"
	"github.com/newrelic/nri-kubernetes/v3/src/kubelet/client"
	"github.com/newrelic/nri-kubernetes/v3/src/kubelet/metric/testdata"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
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
	{MetricName: "container_cpu_cfs_periods_total"},
	{MetricName: "container_cpu_cfs_throttled_periods_total"},
	{MetricName: "container_cpu_cfs_throttled_seconds_total"},
	{MetricName: "container_memory_mapped_file"},
}

func readerToHandler(r io.Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		io.Copy(w, r) // nolint: errcheck
	}
}

func runCAdvisorFetchFunc(t *testing.T, file string) {
	f, err := os.Open(file)
	require.NoError(t, err)

	defer f.Close()

	c := &testClient{
		handler: readerToHandler(f),
	}

	kubeletClient, err := client.New(client.StaticConnector(c, url.URL{}))

	require.NoError(t, err)

	g, err := CadvisorFetchFunc(kubeletClient.MetricFamiliesGetFunc(KubeletCAdvisorMetricsPath), cadvisorQueries)()

	assert.NoError(t, err)
	assert.Equal(t, testdata.ExpectedCadvisorRawData, g)
}

func TestCadvisorFetchFunc(t *testing.T) {
	t.Run("Kubernetes version 1.15", func(t *testing.T) {
		runCAdvisorFetchFunc(t, "./testdata/k8s_v1_15_kubelet_metrics_cadvisor_payload_plain.txt")
	})
	t.Run("Kubernetes version 1.16", func(t *testing.T) {
		runCAdvisorFetchFunc(t, "./testdata/k8s_v1_16_kubelet_metrics_cadvisor_payload_plain.txt")
	})
}

func TestCadvisorFetchFunc_MissingLabels(t *testing.T) {
	f := strings.NewReader(`
container_memory_usage_bytes{id="/kubepods/podf89b6c09-11a3-11e8-a084-080027352a02/3328c17bfd22f1a82fcdf8707c2f8f040c462e548c24780079bba95d276d93e1",image="gcr.io/google_containers/addon-resizer@sha256:e77acf80697a70386c04ae3ab494a7b13917cb30de2326dcf1a10a5118eddabe",name="k8s_addon-resizer_kube-state-metrics-57f4659995-6n2qq_kube-system_f89b6c09-11a3-11e8-a084-080027352a02_17",namespace="kube-system",pod_name="kube-state-metrics-57f4659995-6n2qq"} 1.7788928e+07
container_memory_usage_bytes{container_name="dnsmasq",id="/kubepods/burstable/podbb3b914c-11a3-11e8-a084-080027352a02/81de1e9aba1c051a2f9780a5db594a899c9e4e76613d4c95da4561cc48e8658f",image="sha256:459944ce8cc4f08ebade5c05bb884e4da053d73e61ec6afe82a0b1687317254c",name="k8s_dnsmasq_kube-dns-54cccfbdf8-dznm7_kube-system_bb3b914c-11a3-11e8-a084-080027352a02_13",pod_name="kube-dns-54cccfbdf8-dznm7"} 1.4655488e+07
container_memory_usage_bytes{container_name="grafana",id="/kubepods/besteffort/podbb233a6b-11a3-11e8-a084-080027352a02/7f092105225a729f4917aa6950b5b90236c720fc411eee80ba9f7ca0f639525f",image="k8s.gcr.io/heapster-grafana-amd64@sha256:4a472eb4df03f4f557d80e7c6b903d9c8fe31493108b99fbd6da6540b5448d70",name="k8s_grafana_influxdb-grafana-rsmwp_kube-system_bb233a6b-11a3-11e8-a084-080027352a02_17",namespace="kube-system"} 2.5956352e+07
container_memory_usage_bytes{container_name="heapster",image="k8s.gcr.io/heapster-amd64@sha256:da3288b0fe2312c621c2a6d08f24ccc56183156ec70767987501287db4927b9d",name="k8s_heapster_heapster-5mz5f_kube-system_bb142c1b-11a3-11e8-a084-080027352a02_15",namespace="kube-system",pod_name="heapster-5mz5f"} 3.0572544e+07
container_memory_usage_bytes{container_name="influxdb",id="/kubepods/besteffort/podbb233a6b-11a3-11e8-a084-080027352a02/fd0ca055e308e5d11b0c8fbf273b733d1166aa2823bf7fd724a6b70c72959774",name="k8s_influxdb_influxdb-grafana-rsmwp_kube-system_bb233a6b-11a3-11e8-a084-080027352a02_17",namespace="kube-system",pod_name="influxdb-grafana-rsmwp"} 7.4510336e+07
`)

	c := &testClient{
		handler: readerToHandler(f),
	}

	kubeletClient, err := client.New(client.StaticConnector(c, url.URL{}))
	require.NoError(t, err)

	_, err = CadvisorFetchFunc(kubeletClient.MetricFamiliesGetFunc(KubeletCAdvisorMetricsPath), cadvisorQueries)()
	assert.Error(t, err)

	expectedErrs := []error{
		errors.New("container name not found in cAdvisor metrics"),
		errors.New("namespace not found in cAdvisor metrics"),
		errors.New("pod name not found in cAdvisor metrics"),
		errors.New("container id not found in cAdvisor metrics"),
		errors.New("container image not found in cAdvisor metrics"),
	}

	assert.ElementsMatch(t, expectedErrs, err.(data.ErrorGroup).Errors)
}

func TestExtractContainerIdWithoutSystemD_ReturnsCorrectId(t *testing.T) {
	// given
	fullContainerID := "/docker/d44b560aba016229fd4f87a33bf81e8eaf6c81932a0623530456e8f80f9675ad/kubepods/besteffort/pod6edbcc6c66e4b5af53005f91bf0bc1fd/7588a02459ef3166ba043c5a605c9ce65e4dd250d7ee40428a28d806c4116e97"
	expected := "7588a02459ef3166ba043c5a605c9ce65e4dd250d7ee40428a28d806c4116e97"

	// when
	actual := extractContainerID(fullContainerID)

	assert.Equal(t, expected, actual)
}

func TestExtractContainerIdWithSystemD_ReturnsCorrectId(t *testing.T) {
	// given
	fullContainerID := "/docker/ae17ce6dcd2f27905cedf80609044290eccd98115b4e1ded08fcf6852cf939ae/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod13118b761000f8fe2c4662d5f32d9532.slice/crio-ebccdd64bb3ef5dfa9d9b167cb5e30f9b696c2694fb7e0783af5575c28be3d1b.scope"
	expected := "ebccdd64bb3ef5dfa9d9b167cb5e30f9b696c2694fb7e0783af5575c28be3d1b"

	// when
	actual := extractContainerID(fullContainerID)

	assert.Equal(t, expected, actual)
}

func TestExtractContainerIdDockerGeneric_ReturnsCorrectId(t *testing.T) {
	// given
	fullContainerID := "docker://5a4eefc5b30f6f12402665bd920ee8e889ba9e14bdcc623e2865a79d40f58412"
	expected := "5a4eefc5b30f6f12402665bd920ee8e889ba9e14bdcc623e2865a79d40f58412"

	// when
	actual := extractContainerID(fullContainerID)

	assert.Equal(t, expected, actual)
}
