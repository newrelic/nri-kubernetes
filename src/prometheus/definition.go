package prometheus

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/newrelic/nri-kubernetes/src/definition"
	model "github.com/prometheus/client_model/go"
)

// ControlPlaneComponentTypeGenerator generates the entity type of a
// control plane component.
var ControlPlaneComponentTypeGenerator = func(
	groupLabel string,
	_ string,
	_ definition.RawGroups,
	clusterName string,
) (string, error) {
	return fmt.Sprintf("k8s:%s:controlplane:%s", clusterName, groupLabel), nil
}

// FromRawEntityIDGenerator generates the entity type of a
// control plane component.
var FromRawEntityIDGenerator = func(_, rawEntityID string, _ definition.RawGroups) (string, error) {
	return rawEntityID, nil
}

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

// GroupEntityMetricsBySpec groups metrics coming from Prometheus by the
// given rawEntityID and metric spec.
//
// It differes from GroupMetricsBySpec in that the key that maps to the
// RawMetrics is always the given rawEntityID and that the RawValues
// are of the form []Metric instead of Metric.
//
// Using the given rawEntityID as the entity key for the metrics is
// useful in cases when all the []MetricFamily belong to the same
// entity and there is no way to infer that from the metrics. A good
// example is querying the metrics endpoint of a control plane component.
//
// The resulting RawGroups are of the form:
// {
//   groupLabel: {
//     rawEntityID: {
//       metric_name: [ Metric1, Metric2, ..., Metricn ]
//     }
//   }
// }
func GroupEntityMetricsBySpec(
	specs definition.SpecGroups,
	families []MetricFamily,
	rawEntityID string,
) (g definition.RawGroups, errs []error) {
	g = make(definition.RawGroups)
	for groupLabel := range specs {
		for _, f := range families {
			for _, m := range f.Metrics {
				if _, ok := g[groupLabel]; !ok {
					g[groupLabel] = make(map[string]definition.RawMetrics)
				}

				if _, ok := g[groupLabel][rawEntityID]; !ok {
					g[groupLabel][rawEntityID] = make(definition.RawMetrics)
				}

				groupedMetrics, ok := g[groupLabel][rawEntityID][f.Name]
				if !ok {
					groupedMetrics = make([]Metric, 0)
				}
				g[groupLabel][rawEntityID][f.Name] = append(groupedMetrics.([]Metric), m)
			}
		}

		if len(g[groupLabel]) == 0 {
			errs = append(errs, fmt.Errorf("no data found for %s object", groupLabel))
			continue
		}
	}

	return g, errs
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

// LabelsFilter are functions used to filter labels when executing some
// definition.FetchFunc.
type LabelsFilter func(Labels) Labels

// IgnoreLabelsFilter returns a function that filters-out the given labels.
func IgnoreLabelsFilter(labelsToIgnore ...string) func(Labels) Labels {
	return func(labels Labels) Labels {
		filteredLabels := make(Labels)
		for label, value := range labels {
			ignore := false
			for _, labelToIgnore := range labelsToIgnore {
				if labelToIgnore == label {
					ignore = true
					break
				}
			}
			if !ignore {
				filteredLabels[label] = value
			}
		}
		return filteredLabels
	}
}

// IncludeOnlyLabelsFilter returns a function that filters-out all but the
// given labels.
func IncludeOnlyLabelsFilter(labelsToInclude ...string) func(Labels) Labels {
	return func(labels Labels) Labels {
		filteredLabels := make(Labels)
		for label, value := range labels {
			include := false
			for _, labelToIgnore := range labelsToInclude {
				if labelToIgnore == label {
					include = true
					break
				}
			}
			if include {
				filteredLabels[label] = value
			}
		}
		return filteredLabels
	}
}

// attributeName genereates the attribute name by suffixing the time-series
// labels to the given metricName in order.
func attributeName(metricName, nameOverride string, labels Labels, labelsFilter ...LabelsFilter) string {
	for _, filter := range labelsFilter {
		labels = filter(labels)
	}

	if nameOverride != "" {
		return suffixLabelsInOrder(nameOverride, labels)
	}

	return suffixLabelsInOrder(metricName, labels)
}

// fetchedValuesFromRawMetrics generates a mapping of metrics to `FetchedValue`.
// The metric names used in the mapping are generated for every `Metric` in
// `metrics` by suffixing `Metric.Labels` to `metricName` like:
//
// <metric_name>_<label_1>_<label_1_value>_..._<label_n>_<label_n_value>
//
// Given a `metricName=my_metric` and the following `metrics`:
//
// [
//   {Value: 4, Labels: [{Name: "l1", Value: "a"}, {Name: "l2", Value: "b"}]},
//   {Value: 6, Labels: [{Name: "l1", Value: "c"}, {Name: "l2", Value: "d"}]}
// ]
//
// The following `FetchedValues` will be returned:
//
// {
//    "my_metric_l1_a_l2_b": 4,
//    "my_metric_l1_c_l2_d": 6
// }
//
// The labels used in generating the resulting metrics names can be filtered
// by using `LabelsFilter`. In the case where multiple metrics generate the
// same key to use in the `FetchedValues` mapping, the values will be summed.
//
// Given a `metricName=my_metric` the following `metrics`:
//
// [
//   {Value: 4, Labels: [{Name: "l1", Value: "a"}, {Name: "l2", Value: "b"}]},
//   {Value: 6, Labels: [{Name: "l1", Value: "a"}, {Name: "l2", Value: "d"}]}
// ]
//
// And using IgnoreLabelsFilter("l2") to filter the `l2` label
//
// The following `FetchedValues` will be returned:
//
// {
//    "my_metric_l1_a": 10,
// }
func fetchedValuesFromRawMetrics(
	metricName string,
	nameOverride string,
	metrics []Metric,
	labelsFilter ...LabelsFilter,
) (definition.FetchedValues, error) {
	val := make(definition.FetchedValues)
	for _, metric := range metrics {
		attrName := attributeName(metricName, nameOverride, metric.Labels, labelsFilter...)
		aggregatedValue, ok := val[attrName]

		if !ok {
			val[attrName] = metric.Value
			continue
		}

		switch metric.Value.(type) {
		case CounterValue:
			aggregatedCounter, ok := aggregatedValue.(CounterValue)
			if !ok {
				return nil, fmt.Errorf(
					"incompatible metric type for %s aggregation. Expected: CounterValue. Got: %T",
					metricName,
					metric.Value,
				)
			}
			val[attrName] = aggregatedCounter + metric.Value.(CounterValue)
		case GaugeValue:
			aggregatedCounter, ok := aggregatedValue.(GaugeValue)
			if !ok {
				return nil, fmt.Errorf(
					"incompatible metric type for %s aggregation. Expected: GaugeValue. Got: %T",
					metricName,
					metric.Value,
				)
			}
			val[attrName] = aggregatedCounter + metric.Value.(GaugeValue)
		}
	}
	return val, nil
}

// FromValue creates a FetchFunc that fetches values from prometheus metrics values.
func FromValue(metricName string, labelsFilter ...LabelsFilter) definition.FetchFunc {
	return FromValueWithOverriddenName(metricName, "", labelsFilter...)
}

// FromValueWithOverriddenName creates a FetchFunc that fetches values from prometheus metrics values.
// If there are multiple values returned, and nameOverride is not empty, this name will be used as a prefix instead of the metricName.
func FromValueWithOverriddenName(metricName string, nameOverride string, labelsFilter ...LabelsFilter) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		value, err := definition.FromRaw(metricName)(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		switch m := value.(type) {
		case Metric:
			return m.Value, nil
		case []Metric:
			return fetchedValuesFromRawMetrics(metricName, nameOverride, m, labelsFilter...)
		}
		return nil, fmt.Errorf(
			"incompatible metric type for %s. Expected: Metric or []Metric. Got: %T",
			metricName,
			value,
		)
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

// suffixLabelsInOrder takes the given metricName and appends, in alphabetical
// order, the given labels in the form of <label_key>_<label_value>.
func suffixLabelsInOrder(metricName string, labels Labels) string {

	orderedLabels := make([]string, 0, len(labels))
	for labelKey, labelVal := range labels {
		orderedLabels = append(orderedLabels, fmt.Sprintf("%s_%s", labelKey, labelVal))
	}

	sort.Strings(orderedLabels)

	for _, l := range orderedLabels {
		metricName = fmt.Sprintf("%s_%s", metricName, l)
	}
	return metricName
}

// FromSummary creates a FetchFunc that fetches values from prometheus
// histogram.
//
// It will create one attribute for the count, one for the sum and one per
// quantile. The attributes names will be generated by suffixing the
// time-series labels to the given key, and by suffixing and identifier
// for type of the time-series in relation to the summary (count, sum or quantile).
//
// - <metric_name>_<label_1>_<label_1_value>_..._<label_n>_<label_n_value>_sum
// - <metric_name>_<label_1>_<label_1_value>_..._<label_n>_<label_n_value>_count
// - <metric_name>_<label_1>_<label_1_value>_..._<label_n>_<label_n_value>_quantile_<quantile_dimention_1>
// - ...
// - <metric_name>_<label_1>_<label_1_value>_..._<label_n>_<label_n_value>_quantile_<quantile_dimention_n>
//
// Since it expects the RawValue to be of type []Metric it should be
// used when grouping with GroupEntityMetricsBySpec.
func FromSummary(key string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		value, err := definition.FromRaw(key)(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		metrics, ok := value.([]Metric)
		if !ok {
			return nil, fmt.Errorf(
				"incompatible metric type for %s. Expected: []Metric. Got: %T",
				key,
				value,
			)
		}

		val := make(definition.FetchedValues)
		for _, metric := range metrics {
			summary, ok := metric.Value.(*model.Summary)
			if !ok {
				return nil, fmt.Errorf(
					"incompatible metric type for %s. Expected: Summary. Got: %T",
					key,
					metric.Value,
				)
			}
			name := suffixLabelsInOrder(key, metric.Labels)
			val[fmt.Sprintf("%s_count", name)] = summary.GetSampleCount()

			sumVal := summary.GetSampleSum()
			if validNRValue(sumVal) {
				val[fmt.Sprintf("%s_sum", name)] = sumVal
			}

			for _, q := range summary.GetQuantile() {
				quantileVal := q.GetValue()
				if validNRValue(quantileVal) {
					nameWithQuantileSuffix := fmt.Sprintf(
						"%s_quantile_%s",
						name,
						strconv.FormatFloat(q.GetQuantile(), 'f', -1, 64),
					)
					val[nameWithQuantileSuffix] = quantileVal
				}
			}
		}
		return val, nil
	}
}

// validNRValue returns if v is a New Relic metric supported float64.
func validNRValue(v float64) bool {
	return !math.IsInf(v, 0) && !math.IsNaN(v)
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

// metricContainsLabels returns true is the metric contains the given labels,
// false otherwise.
func metricContainsLabels(m Metric, labels ...string) bool {
	for _, k := range labels {
		if _, ok := m.Labels[k]; !ok {
			return false
		}
	}
	return true
}

// getRandomMetricWithLabels returns the first metric that contains the given
// labels.
func getRandomMetricWithLabels(metrics definition.RawMetrics, labels ...string) (metricKey string, value definition.RawValue, err error) {
	found := false
	for metricKey, value = range metrics {
		m, ok := value.(Metric)
		if !ok {
			continue
		}

		if metricContainsLabels(m, labels...) {
			found = true
			break
		}
	}
	if !found {
		err = fmt.Errorf("metric with the labels %v not found", labels)
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

// labelsFromMetric returns the labels of the metric. The labels keys
// are formatted from "<prefix>_" to "<prefix>."
func labelsFromMetric(
	parentGroupLabel string,
	relatedMetricKey string,
	groupLabel string,
	entityID string,
	groups definition.RawGroups,
	prefix string,
) (definition.FetchedValue, error) {
	rawEntityID, err := getRawEntityID(parentGroupLabel, groupLabel, entityID, groups)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot retrieve the entity ID of metrics to inherit labels from, got error: %v",
			err,
		)
	}

	parent, err := fetchMetric(relatedMetricKey)(parentGroupLabel, rawEntityID, groups)
	if err != nil {
		return nil, fmt.Errorf(
			"related metric not found. Metric: %s %s:%s",
			relatedMetricKey,
			parentGroupLabel,
			rawEntityID,
		)
	}

	multiple := make(definition.FetchedValues)
	for k, v := range parent.(Metric).Labels {
		key := fmt.Sprintf(
			"%s.%s",
			prefix,
			strings.TrimPrefix(k, fmt.Sprintf("%s_", prefix)),
		)
		multiple[key] = v
	}

	return multiple, nil

}

// InheritAllLabelsFrom gets all the label values from from a related metric.
// Related metric means any metric you can get with the info that you have in your own metric.
func InheritAllLabelsFrom(parentGroupLabel, relatedMetricKey string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		return labelsFromMetric(parentGroupLabel, relatedMetricKey, groupLabel, entityID, groups, "label")
	}
}

// InheritAllSelectorsFrom gets all the label values from from a related
// metric and changes the prefix "selector_" for "selector.". It's meant to
// be used with metrics that contain label selectors.
// Related metric means any metric you can get with the info that you
// have in your own metric.
func InheritAllSelectorsFrom(parentGroupLabel, relatedMetricKey string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		return labelsFromMetric(parentGroupLabel, relatedMetricKey, groupLabel, entityID, groups, "selector")
	}
}

func getRawEntityID(parentGroupLabel, groupLabel, entityID string, groups definition.RawGroups) (string, error) {
	group, ok := groups[groupLabel][entityID]
	if !ok {
		return "", fmt.Errorf("metrics not found for %v with entity ID: %v", groupLabel, entityID)
	}

	var rawEntityID string
	switch parentGroupLabel {
	case "node", "namespace":
		metricKey, r := getRandomMetric(group)
		m, ok := r.(Metric)

		if !ok {
			return "", fmt.Errorf("incompatible metric type. Expected: Metric. Got: %T", r)
		}

		rawEntityID, ok = m.Labels[parentGroupLabel]

		if !ok {
			return "", fmt.Errorf("label not found. Label: '%s', Metric: %s", parentGroupLabel, metricKey)
		}
	default:
		metricKey, r, err := getRandomMetricWithLabels(group, "namespace", parentGroupLabel)

		if err != nil {
			return "", err
		}

		m, ok := r.(Metric)

		if !ok {
			return "", fmt.Errorf("incompatible metric type. Expected: Metric. Got: %T", r)
		}

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
