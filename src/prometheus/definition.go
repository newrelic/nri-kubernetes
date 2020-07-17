package prometheus

import (
	"errors"
	"fmt"
	"strings"

	"github.com/newrelic/nri-kubernetes/src/definition"
)

// FromLabelValueEntityTypeGenerator generates the entity type using the cluster name and group label.
// If group label is different than "namespace" or "node", then entity type is also composed of namespace.
// If group label is "container" then pod name is also included.
func FromLabelValueEntityTypeGenerator(key string) definition.EntityTypeGeneratorFunc {
	return func(groupLabel string, rawEntityID string, g definition.RawGroups, clusterName string) (string, error) {

		switch groupLabel {
		case "namespace", "node":
			return fmt.Sprintf("k8s:%s:%s", clusterName, groupLabel), nil

		case "container":
			labels, err := getLabels(groupLabel, rawEntityID, key, g, "namespace", "pod")
			if err != nil {
				return "", err
			}
			if len(labels) != 2 {
				return "", fmt.Errorf("cannot retrieve values for composing entity type for %q", groupLabel)
			}
			namespace := labels[0]
			podName := labels[1]
			if namespace == "" || podName == "" {
				return "", fmt.Errorf("empty values for generated entity type for %q", groupLabel)
			}
			return fmt.Sprintf("k8s:%s:%s:%s:%s", clusterName, namespace, podName, groupLabel), nil

		default:
			labels, err := getLabels(groupLabel, rawEntityID, key, g, "namespace")
			if err != nil {
				return "", err
			}
			if len(labels) == 0 {
				return "", fmt.Errorf("cannot retrieve values for composing entity type for %q", groupLabel)
			}
			namespace := labels[0]

			if namespace == "" {
				return "", fmt.Errorf("empty namespace for generated entity type for %q", groupLabel)
			}
			return fmt.Sprintf("k8s:%s:%s:%s", clusterName, namespace, groupLabel), nil
		}
	}
}

func getLabels(groupLabel, rawEntityID, key string, groups definition.RawGroups, labels ...string) ([]string, error) {
	var s []string
	for _, label := range labels {
		v, err := FromLabelValue(key, label)(groupLabel, rawEntityID, groups)
		if err != nil {
			return s, fmt.Errorf("cannot fetch label %s for metric %s, %s", label, key, err)
		}
		if v == nil {
			return s, fmt.Errorf("label %s not found for metric %s", label, key)

		}

		val, ok := v.(string)
		if !ok {
			return s, fmt.Errorf("incorrect type of label %s for metric %s", label, key)
		}
		s = append(s, val)
	}
	return s, nil
}

// FromLabelValueEntityIDGenerator generates an entityID using the value of the specified label
// for the given metric key.
func FromLabelValueEntityIDGenerator(key, label string) definition.EntityIDGeneratorFunc {
	return func(groupLabel string, rawEntityID string, g definition.RawGroups) (string, error) {
		v, err := FromLabelValue(key, label)(groupLabel, rawEntityID, g)
		if err != nil {

			return "", fmt.Errorf("cannot fetch label %s for metric %s, %s", label, key, err)
		}

		if v == nil {
			return "", fmt.Errorf("incorrect value of fetched data for metric %s", key)
		}

		val, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("incorrect type of fetched data for metric %s", key)
		}

		return val, err
	}
}

// FromLabelsValueEntityIDGeneratorForPendingPods generates entity ID for a pod in pending status,
// which is not scheduled. Otherwise entity ID is not generated. This is due to the fact that
// Kubelet /pods endpoint does not have information about those pods. The rest of the pods
// is reported from Kubelet /pods endpoint.
func FromLabelsValueEntityIDGeneratorForPendingPods() definition.EntityIDGeneratorFunc {
	return func(groupLabel string, rawEntityID string, g definition.RawGroups) (string, error) {
		podName, err := FromLabelValueEntityIDGenerator("kube_pod_status_phase", "pod")(groupLabel, rawEntityID, g)
		if err != nil {
			return "", err
		}

		isScheduled, err := FromLabelValueEntityIDGenerator("kube_pod_status_scheduled", "condition")(groupLabel, rawEntityID, g)
		if err != nil {
			return "", err
		}
		if isScheduled != "false" {
			return "", fmt.Errorf("ignoring pending pod, which is scheduled: reported from Kubelet endpoint")
		}

		return podName, nil
	}
}

// GroupMetricsBySpec groups metrics coming from Prometheus by a given metric spec.
// Example: grouping by K8s pod, container, etc.
func GroupMetricsBySpec(specs definition.SpecGroups, families []MetricFamily) (g definition.RawGroups, errs []error) {
	g = make(definition.RawGroups)
	for groupLabel := range specs {
		for _, f := range families {
			for _, m := range f.Metrics {
				if !m.Labels.Has(groupLabel) {
					continue
				}

				var rawEntityID string
				switch groupLabel {
				case "namespace", "node":
					rawEntityID = m.Labels[groupLabel]
				case "container":
					rawEntityID = fmt.Sprintf("%v_%v_%v", m.Labels["namespace"], m.Labels["pod"], m.Labels[groupLabel])
				default:
					rawEntityID = fmt.Sprintf("%v_%v", m.Labels["namespace"], m.Labels[groupLabel])
				}

				if _, ok := g[groupLabel]; !ok {
					g[groupLabel] = make(map[string]definition.RawMetrics)
				}

				if _, ok := g[groupLabel][rawEntityID]; !ok {
					g[groupLabel][rawEntityID] = make(definition.RawMetrics)
				}

				g[groupLabel][rawEntityID][f.Name] = m
			}
		}

		if len(g[groupLabel]) == 0 {
			errs = append(errs, fmt.Errorf("no data found for %s object", groupLabel))
			continue
		}
	}

	return g, errs
}

// FromValue creates a FetchFunc that fetches values from prometheus metrics values.
func FromValue(key string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		value, err := definition.FromRaw(key)(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		v, ok := value.(Metric)
		if !ok {
			return nil, fmt.Errorf("incompatible metric type. Expected: Metric. Got: %T", value)
		}

		return v.Value, nil
	}
}

// FromLabelValue creates a FetchFunc that fetches values from prometheus metrics labels.
func FromLabelValue(key, label string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		value, err := definition.FromRaw(key)(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		v, ok := value.(Metric)
		if !ok {
			return nil, fmt.Errorf("incompatible metric type. Expected: Metric. Got: %T", value)
		}

		l, ok := v.Labels[label]
		if !ok {
			return nil, errors.New("label not found in prometheus metric")
		}

		return l, nil
	}
}

func getRandomMetric(metrics definition.RawMetrics) (metricKey string, value definition.RawValue) {
	for metricKey, value = range metrics {
		if _, ok := value.(Metric); !ok {
			continue
		}
		// We just want 1.
		break
	}

	return
}

func fetchMetric(metricKey string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {

		value, err := definition.FromRaw(metricKey)(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		v, ok := value.(Metric)
		if !ok {
			return nil, fmt.Errorf("incompatible metric type. Expected: Metric. Got: %T", value)
		}

		return v, nil
	}
}

// InheritSpecificLabelValuesFrom gets the specified label values from a related metric.
// Related metric means any metric you can get with the info that you have in your own metric.
func InheritSpecificLabelValuesFrom(parentGroupLabel, relatedMetricKey string, labelsToRetrieve map[string]string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		rawEntityID, err := getRawEntityID(parentGroupLabel, groupLabel, entityID, groups)
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve the entity ID of metrics to inherit value from, got error: %v", err)
		}
		parent, err := definition.FromRaw(relatedMetricKey)(parentGroupLabel, rawEntityID, groups)
		if err != nil {
			return nil, fmt.Errorf("related metric not found. Metric: %s %s:%s", relatedMetricKey, parentGroupLabel, rawEntityID)
		}

		multiple := make(definition.FetchedValues)
		for k, v := range parent.(Metric).Labels {
			for n, l := range labelsToRetrieve {
				if l == k {
					multiple[n] = v
				}
			}
		}

		return multiple, nil
	}
}

// InheritAllLabelsFrom gets all the label values from from a related metric.
// Related metric means any metric you can get with the info that you have in your own metric.
func InheritAllLabelsFrom(parentGroupLabel, relatedMetricKey string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		rawEntityID, err := getRawEntityID(parentGroupLabel, groupLabel, entityID, groups)
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve the entity ID of metrics to inherit labels from, got error: %v", err)
		}

		parent, err := fetchMetric(relatedMetricKey)(parentGroupLabel, rawEntityID, groups)
		if err != nil {
			return nil, fmt.Errorf("related metric not found. Metric: %s %s:%s", relatedMetricKey, parentGroupLabel, rawEntityID)
		}

		multiple := make(definition.FetchedValues)
		for k, v := range parent.(Metric).Labels {
			multiple[fmt.Sprintf("label.%v", strings.TrimPrefix(k, "label_"))] = v
		}

		return multiple, nil
	}
}

func getRawEntityID(parentGroupLabel, groupLabel, entityID string, groups definition.RawGroups) (string, error) {
	group, ok := groups[groupLabel][entityID]
	if !ok {
		return "", fmt.Errorf("metrics not found for %v with entity ID: %v", groupLabel, entityID)
	}
	metricKey, r := getRandomMetric(group)
	m, ok := r.(Metric)

	if !ok {
		return "", fmt.Errorf("incompatible metric type. Expected: Metric. Got: %T", r)
	}

	var rawEntityID string
	switch parentGroupLabel {
	case "node", "namespace":
		rawEntityID, ok = m.Labels[parentGroupLabel]
		if !ok {
			return "", fmt.Errorf("label not found. Label: '%s', Metric: %s", parentGroupLabel, metricKey)
		}
	default:
		namespaceID, ok := m.Labels["namespace"]
		if !ok {
			return "", fmt.Errorf("label not found. Label: 'namespace', Metric: %s", metricKey)
		}
		relatedMetricID, ok := m.Labels[parentGroupLabel]
		if !ok {
			return "", fmt.Errorf("label not found. Label: %s, Metric: %s", parentGroupLabel, metricKey)
		}
		rawEntityID = fmt.Sprintf("%v_%v", namespaceID, relatedMetricID)
	}
	return rawEntityID, nil
}
