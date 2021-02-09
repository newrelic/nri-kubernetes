package metric

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/data"
	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
)

const (
	// KubeletCAdvisorMetricsPath is the path where kubelet serves information about cadvisor.
	KubeletCAdvisorMetricsPath = "/metrics/cadvisor"

	// StandaloneCAdvisorMetricsPath is the path where standalone cadvisor serves information.
	StandaloneCAdvisorMetricsPath = "/metrics"
)

var (
	dockerNativeWithoutSystemD = regexp.MustCompile(`^.*([0-9a-f]+)$`)
	dockerNativeWithSystemD    = regexp.MustCompile(`^.*\w+-([0-9a-f]+)\.scope$`)
	dockerGeneric              = regexp.MustCompile(`^([0-9a-f]+)$`)
)

// getLabel returns the first label it finds by the given names
func getLabel(labels prometheus.Labels, names ...string) (string, bool) {
	for _, name := range names {
		if labels.Has(name) {
			return labels[name], true
		}
	}
	return "", false
}

// CadvisorFetchFunc creates a FetchFunc that fetches data from the kubelet cadvisor metrics path.
func CadvisorFetchFunc(c client.HTTPClient, queries []prometheus.Query) data.FetchFunc {
	return func() (definition.RawGroups, error) {
		families, err := prometheus.Do(c, KubeletCAdvisorMetricsPath, queries)
		if err != nil {
			return nil, fmt.Errorf("error requesting cadvisor metrics endpoint. %s. Try setting the CADVISOR_PORT env variable in the configuration", err)
		}

		var errs []error

		g := definition.RawGroups{
			"container": make(map[string]definition.RawMetrics),
		}

		for _, f := range families {
			for _, m := range f.Metrics {

				if label, _ := getLabel(m.Labels, "container_name", "container"); label == "POD" {
					// skipping metrics from pod containers
					continue
				}

				rawEntityID, err := createRawEntityID(m)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				// It does not belong to a container that we care about. Special case that does not warrant an error
				if rawEntityID == "" {
					continue
				}

				containerID := extractContainerID(m.Labels["id"])
				if containerID == "" {
					errs = append(errs, errors.New("container id not found in cAdvisor metrics"))
					continue
				}

				var (
					metrics definition.RawMetrics
					ok      bool
				)

				if metrics, ok = g["container"][rawEntityID]; !ok {
					metrics = make(definition.RawMetrics)
					metrics["containerID"] = containerID
					g["container"][rawEntityID] = metrics
				}

				switch f.Name {
				case "container_memory_usage_bytes":
					// Special case, where we are only interested in collecting containerId and containerImageID
					if m.Labels["image"] == "" {
						errs = append(errs, errors.New("container image not found in cAdvisor metrics"))
						continue
					}
					metrics["containerImageID"] = m.Labels["image"]
				default:
					// by default, we want the actual metric
					metrics[f.Name] = m.Value
				}
			}
		}

		if len(errs) > 0 {
			return g, data.ErrorGroup{
				Errors:      errs,
				Recoverable: true,
			}
		}

		return g, nil
	}
}

func createRawEntityID(m prometheus.Metric) (string, error) {
	containerName, ok := getLabel(m.Labels, "container_name", "container")
	if !ok {
		return "", errors.New("container name not found in cAdvisor metrics")
	}
	if containerName == "" {
		return "", nil
	}

	namespace := m.Labels["namespace"]
	if namespace == "" {
		return "", errors.New("namespace not found in cAdvisor metrics")

	}

	podName, _ := getLabel(m.Labels, "pod_name", "pod")
	if podName == "" {
		return "", errors.New("pod name not found in cAdvisor metrics")
	}

	return fmt.Sprintf("%s_%s_%s", namespace, podName, containerName), nil
}

// /kubepods/besteffort/podba8b34d7-11a3-11e8-a084-080027352a02/a949bd136c1397b9f52905538ee11450427be33648abe38db06be2e5cfbeca49
// /docker/ae17ce6dcd2f27905cedf80609044290eccd98115b4e1ded08fcf6852cf939ae/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod13118b761000f8fe2c4662d5f32d9532.slice/crio-ebccdd64bb3ef5dfa9d9b167cb5e30f9b696c2694fb7e0783af5575c28be3d1b.scope
// /docker/d44b560aba016229fd4f87a33bf81e8eaf6c81932a0623530456e8f80f9675ad/kubepods/besteffort/pod6edbcc6c66e4b5af53005f91bf0bc1fd/7588a02459ef3166ba043c5a605c9ce65e4dd250d7ee40428a28d806c4116e97
func extractContainerID(v string) string {
	containerID := v[strings.LastIndex(v, "/")+1:]
	matches := dockerNativeWithSystemD.FindStringSubmatch(containerID)
	if len(matches) > 0 {
		return matches[1]
	}
	matches = dockerNativeWithoutSystemD.FindStringSubmatch(containerID)
	if len(matches) > 0 {
		return matches[0]
	}
	matches = dockerGeneric.FindStringSubmatch(containerID)
	if len(matches) > 0 {
		return matches[0]
	}
	return containerID
}
