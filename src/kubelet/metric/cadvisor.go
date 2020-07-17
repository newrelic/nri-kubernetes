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
			if f.Name == "container_memory_usage_bytes" {
				for _, m := range f.Metrics {

					// containerName is used for generating the rawEntityID
					containerName := m.Labels["container_name"]
					if containerName == "POD" {
						// skipping metrics from pod containers
						continue
					}

					if containerName == "" {
						errs = append(errs, errors.New("container name not found in cadvisor metrics"))
						continue
					}

					// namespace is used for generating the rawEntityID
					namespace := m.Labels["namespace"]
					if namespace == "" {
						errs = append(errs, errors.New("namespace not found in cadvisor metrics"))
						continue
					}

					// namespace is used for generating the rawEntityID
					podName := m.Labels["pod_name"]
					if podName == "" {
						errs = append(errs, errors.New("pod name not found in cadvisor metrics"))
						continue
					}

					rawEntityID := fmt.Sprintf("%s_%s_%s", namespace, podName, containerName)

					container := make(definition.RawMetrics, 2)

					if v := extractContainerID(m.Labels["id"]); v == "" {
						errs = append(errs, errors.New("container id not found in cadvisor metrics"))
						continue
					} else {
						container["containerID"] = v
					}

					if m.Labels["image"] == "" {
						errs = append(errs, errors.New("container image not found in cadvisor metrics"))
						continue
					}

					container["containerImageID"] = m.Labels["image"]

					g["container"][rawEntityID] = container
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

// /kubepods/besteffort/podba8b34d7-11a3-11e8-a084-080027352a02/a949bd136c1397b9f52905538ee11450427be33648abe38db06be2e5cfbeca49
func extractContainerID(v string) string {
	return v[strings.LastIndex(v, "/")+1:]
}
