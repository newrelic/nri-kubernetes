package metric

import (
	"errors"
	"fmt"
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
func extractContainerID(v string) string {
	return v[strings.LastIndex(v, "/")+1:]
}
