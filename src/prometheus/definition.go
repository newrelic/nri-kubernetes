package prometheus

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	model "github.com/prometheus/client_model/go"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

var (
	ErrExpectedLabelsNotFound     = errors.New("expected labels not found")
	ErrUnexpectedEmptyLabels      = errors.New("unexpected empty labels")
	ErrLabelNotFound              = errors.New("label not found on metric")
	ErrMetricSliceEmpty           = errors.New("metric slice for key was empty")
	ErrLabelNotFoundInFirstMetric = errors.New("label not found in the first metric for key")
	ErrIncompatibleMetricType     = errors.New("incompatible metric type for key")
	ErrExpectedMetricType         = errors.New("expected metric type for key to be Metric")
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
		return defaultEntityTypeFromLabelValue(key, groupLabel, groupLabel, rawEntityID, g, clusterName)
	}
}

// FromLabelValueEntityTypeGeneratorWithCustomGroup generates the entity type in the same way
// `FromLabelValueEntityTypeGenerator` does, but it uses the provided group instead of the group label to compose it.
func FromLabelValueEntityTypeGeneratorWithCustomGroup(key string, group string) definition.EntityTypeGeneratorFunc {
	return func(groupLabel string, rawEntityID string, g definition.RawGroups, clusterName string) (string, error) {
		return defaultEntityTypeFromLabelValue(key, group, groupLabel, rawEntityID, g, clusterName)
	}
}

func defaultEntityTypeFromLabelValue( //nolint: cyclop
	key string, group, groupLabel string, rawEntityID string, g definition.RawGroups, clusterName string,
) (string, error) {
	switch groupLabel {
	case "namespace", "node", "persistentvolume": //nolint: goconst
		return fmt.Sprintf("k8s:%s:%s", clusterName, group), nil

	case "container":
		labels, err := getLabels(groupLabel, rawEntityID, key, g, "namespace", "pod")
		if err != nil {
			return "", err
		}
		if neededLabels := 2; len(labels) != neededLabels {
			return "", fmt.Errorf("%w: cannot retrieve values for composing entity type for %q", ErrExpectedLabelsNotFound, groupLabel)
		}
		namespace := labels[0]
		podName := labels[1]
		if namespace == "" || podName == "" {
			return "", fmt.Errorf("%w: empty values for generated entity type for %q", ErrUnexpectedEmptyLabels, groupLabel)
		}
		return fmt.Sprintf("k8s:%s:%s:%s:%s", clusterName, namespace, podName, group), nil

	default:
		labels, err := getLabels(groupLabel, rawEntityID, key, g, "namespace")
		if err != nil {
			return "", err
		}
		if len(labels) == 0 {
			return "", fmt.Errorf("%w: cannot retrieve values for composing entity type for %q", ErrExpectedLabelsNotFound, groupLabel)
		}
		namespace := labels[0]

		if namespace == "" {
			return "", fmt.Errorf("%w: empty values for generated entity type for %q", ErrUnexpectedEmptyLabels, groupLabel)
		}
		return fmt.Sprintf("k8s:%s:%s:%s", clusterName, namespace, group), nil
	}
}

func getLabels(groupLabel, rawEntityID, key string, groups definition.RawGroups, labels ...string) ([]string, error) {
	var s []string
	for _, label := range labels {
		v, err := FromLabelValue(key, label)(groupLabel, rawEntityID, groups)
		if err != nil {
			return s, fmt.Errorf("cannot fetch label %q for metric %q: %w", label, key, err)
		}
		if v == nil {
			return s, fmt.Errorf("label %q not found for metric %q", label, key)
		}

		val, ok := v.(string)
		if !ok {
			return s, fmt.Errorf("unexpected type %T of label %q for metric %q", v, label, key)
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
			return "", fmt.Errorf("cannot fetch label %q for metric %q: %w", label, key, err)
		}

		if v == nil {
			return "", fmt.Errorf("unexpected nil value of fetched data for metric %q", key)
		}

		val, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("incorrect type %T of fetched data for metric %q", v, key)
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

func FromLabelGetNamespace(metrics definition.RawMetrics) string {
	for _, metric := range metrics {
		m, ok := metric.(Metric)
		if ok && m.Labels["namespace"] != "" {
			return m.Labels["namespace"]
		}
	}
	return ""
}

// GroupEntityMetricsBySpec groups metrics coming from Prometheus by the
// given rawEntityID and metric spec.
//
// It differs from GroupMetricsBySpec in that the key that maps to the
// RawMetrics is always the given rawEntityID and that the RawValues
// are of the form []Metric instead of Metric.
//
// Using the given rawEntityID as the entity key for the metrics is
// useful in cases when all the []MetricFamily belong to the same
// entity and there is no way to infer that from the metrics. A good
// example is querying the metrics endpoint of a control plane component.
//
// The resulting RawGroups are of the form:
//
//	{
//	  groupLabel: {
//	    rawEntityID: {
//	      metric_name: [ Metric1, Metric2, ..., Metricn ]
//	    }
//	  }
//	}
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

				rawEntityID := ""

				// Skip adding too specific metrics for higher level groups. E.g. don't add Pod metrics to Node group,
				// as there will be 1 to many relationship between them and those metrics will be overwritten anyway,
				// as we use namespace name or node name as a key.
				if groupLabel == "node" && m.Labels.Has("pod") {
					continue
				}

				if groupLabel == "namespace" && (m.Labels.Has("daemonset") ||
					m.Labels.Has("pod") ||
					m.Labels.Has("endpoint") ||
					m.Labels.Has("service") ||
					m.Labels.Has("deployment") || m.Labels.Has("replicaset")) {
					continue
				}

				switch groupLabel {
				case "namespace", "node", "persistentvolume":
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

				if v, ok := g[groupLabel][rawEntityID][f.Name]; ok {
					if oldMetric, ok := v.(Metric); ok {
						g[groupLabel][rawEntityID][f.Name] = []Metric{oldMetric}
					}

					g[groupLabel][rawEntityID][f.Name] = append(g[groupLabel][rawEntityID][f.Name].([]Metric), m)

					continue
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

// IncludeOnlyWhenLabelMatchFilter returns a function that filters-out all but the
// given label-value key pairs.
func IncludeOnlyWhenLabelMatchFilter(labelsToInclude Labels) func(Labels) Labels {
	return func(labels Labels) Labels {
		filteredLabels := make(Labels)
		for label, value := range labels {
			for labelToInclude, valueToInclude := range labelsToInclude {
				if label == labelToInclude && value == valueToInclude {
					filteredLabels[label] = value
					break
				}
			}
		}

		return filteredLabels
	}
}

// attributeName generates the attribute name by suffixing the time-series
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
//
//	{Value: 4, Labels: [{Name: "l1", Value: "a"}, {Name: "l2", Value: "b"}]},
//	{Value: 6, Labels: [{Name: "l1", Value: "c"}, {Name: "l2", Value: "d"}]}
//
// ]
//
// The following `FetchedValues` will be returned:
//
//	{
//	   "my_metric_l1_a_l2_b": 4,
//	   "my_metric_l1_c_l2_d": 6
//	}
//
// The labels used in generating the resulting metrics names can be filtered
// by using `LabelsFilter`. In the case where multiple metrics generate the
// same key to use in the `FetchedValues` mapping, the values will be summed.
//
// Given a `metricName=my_metric` the following `metrics`:
//
// [
//
//	{Value: 4, Labels: [{Name: "l1", Value: "a"}, {Name: "l2", Value: "b"}]},
//	{Value: 6, Labels: [{Name: "l1", Value: "a"}, {Name: "l2", Value: "d"}]}
//
// ]
//
// And using IgnoreLabelsFilter("l2") to filter the `l2` label
//
// The following `FetchedValues` will be returned:
//
//	{
//	   "my_metric_l1_a": 10,
//	}
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

		value, err := getMetricValue(metricName, aggregatedValue, metric)
		if err != nil {
			return nil, err
		}

		val[attrName] = value
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

// FromLabelValue creates a FetchFunc that fetches a value from a Prometheus metric's label.
//
// It is a higher-order function that takes the source metric name (`key`) and the desired
// label name (`label`) and returns a FetchFunc.
//
// The returned function is robust and can handle two types of raw metric data:
//   - A single Metric: It will look for the label on that metric.
//   - A slice of Metrics ([]Metric): It will look for the label on the *first* metric
//     in the slice, assuming the label is consistent across all metrics in the group.
//
// It returns an error if the source metric is not found, if the data is of an
// unexpected type, or if the specified label does not exist on the metric.
func FromLabelValue(key, label string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		value, err := definition.FromRaw(key)(groupLabel, entityID, groups)
		if err != nil {
			return nil, err // Propagate errors from the underlying fetcher.
		}

		var l string
		var ok bool

		switch v := value.(type) {
		case Metric:
			l, ok = v.Labels[label]
			if !ok {
				return nil, fmt.Errorf("label %q not found on metric %q: %w", label, key, ErrLabelNotFound)
			}
		case []Metric:
			if len(v) == 0 {
				return nil, fmt.Errorf("metric slice for key %q was empty: %w", key, ErrMetricSliceEmpty)
			}
			// Assume the label is consistent across all metrics in the slice.
			l, ok = v[0].Labels[label]
			if !ok {
				return nil, fmt.Errorf("label %q not found in the first metric for key %q: %w", label, key, ErrLabelNotFoundInFirstMetric)
			}
		default:
			return nil, fmt.Errorf("incompatible metric type for %q. Expected: Metric or []Metric. Got: %T: %w", key, value, ErrIncompatibleMetricType)
		}

		return l, nil
	}
}

// FromFlattenedMetrics creates a FetchFunc that processes a slice of metrics
// and "unpacks" it into a flat map of metrics.
//
// It is designed for Prometheus metrics where multiple time series represent different
// aspects of a single logical entity, like kube_resourcequota.
//
// Parameters:
//   - metricName: The name of the metric in the RawMetrics map that holds the slice (e.g., "kube_resourcequota").
//   - metricKeyLabel: The label on the source metrics whose value will become the key for the unpacked numeric metrics (e.g., "type", which has values "hard" and "used").
//
// Example:
//
// Given a slice of metrics like:
//
//	Metric{Labels: {"resource": "pods", "type": "hard"}, Value: 10}
//	Metric{Labels: {"resource": "pods", "type": "used"}, Value: 8}
//
// Calling FromFlattenedMetrics("...", "type") will produce:
//
//	{
//	  "hard": 10,
//	  "used": 8,
//	}
func FromFlattenedMetrics(metricName, metricKeyLabel string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		rawMetrics, err := definition.FromRaw(metricName)(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		metrics, ok := rawMetrics.([]Metric)
		if !ok || len(metrics) == 0 {
			// No data to process is not an error.
			return nil, nil
		}

		fetchedValues := make(definition.FetchedValues)

		// Loop through the slice to create the individual numeric metrics.
		for _, m := range metrics {
			metricKey, ok := m.Labels[metricKeyLabel]
			if !ok {
				// Skip any metric in the slice that doesn't have the key label.
				continue
			}
			fetchedValues[metricKey] = m.Value
		}

		return fetchedValues, nil
	}
}

// FromMetricWithPrefixedLabels creates a FetchFunc that gets a single metric
// and extracts all of its Prometheus labels that have a given prefix (e.g., "label_").
// It returns them as attributes, renaming the keys (e.g., "label_foo" -> "label.foo").
func FromMetricWithPrefixedLabels(metricName, prefix string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		rawMetric, err := definition.FromRaw(metricName)(groupLabel, entityID, groups)
		if err != nil {
			return nil, nil //nolint:nilerr // Gracefully handle if the metric is not present.
		}

		metric, ok := rawMetric.(Metric)
		if !ok {
			return nil, fmt.Errorf("expected metric type for %q to be Metric, but got %T: %w", metricName, rawMetric, ErrExpectedMetricType)
		}

		fetchedValues := make(definition.FetchedValues)
		prefixWithUnderscore := prefix + "_"
		for key, value := range metric.Labels {
			if strings.HasPrefix(key, prefixWithUnderscore) {
				attributeName := strings.Replace(key, "_", ".", 1)
				fetchedValues[attributeName] = value
			}
		}

		return fetchedValues, nil
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
// - <metric_name>_<label_1>_<label_1_value>_..._<label_n>_<label_n_value>_quantile_<quantile_dimension_1>
// - ...
// - <metric_name>_<label_1>_<label_1_value>_..._<label_n>_<label_n_value>_quantile_<quantile_dimension_n>
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

// FromValueWithLabelsFilter creates a FetchFunc that fetches values from prometheus metrics values given specific
// labels filter.
func FromValueWithLabelsFilter(metricName string, nameOverride string, labelsFilter ...LabelsFilter) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		value, err := definition.FromRaw(metricName)(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		switch m := value.(type) {
		case Metric:
			return m.Value, nil
		case []Metric:
			return fetchedValuesFromRawMetricsWithLabels(metricName, nameOverride, m, false, labelsFilter...)
		}
		return nil, fmt.Errorf(
			"incompatible metric type for %s. Expected: Metric or []Metric. Got: %T",
			metricName,
			value,
		)
	}
}

// fetchedValuesFromRawMetricsWithLabels generates a mapping of metrics to `FetchedValue` by metricName or nameOverride
// if provided and skips the metrics aggregation if there are no matching labels for that metric.
// In case there aren't any matching filters, the function won't return any value unless returnZeroWhenEmpty is true.
//
// Given a `metricName=my_metric`, a label filter function returning `l3:d` and the following `metrics`:
//
// [
//
//	{Value: 4, Labels: [{Name: "l1", Value: "a"}, {Name: "l2", Value: "b"}]},
//	{Value: 6, Labels: [{Name: "l1", Value: "c"}, {Name: "l3", Value: "d"}]}
//
// ]
//
// The following `FetchedValues` will be returned:
//
//	{
//	   "my_metric": 6,
//	}
func fetchedValuesFromRawMetricsWithLabels(
	metricName string,
	nameOverride string,
	metrics []Metric,
	returnZeroWhenEmpty bool,
	labelsFilter ...LabelsFilter,
) (definition.FetchedValues, error) {
	val := make(definition.FetchedValues)
	for _, metric := range metrics {
		if !hasMatchingLabels(metric, labelsFilter...) {
			continue
		}

		if nameOverride != "" {
			metricName = nameOverride
		}

		aggregatedValue, ok := val[metricName]
		if !ok {
			val[metricName] = metric.Value
			continue
		}

		value, err := getMetricValue(metricName, aggregatedValue, metric)
		if err != nil {
			return nil, err
		}

		val[metricName] = value
	}

	// If no metrics matched the filter and returnZeroWhenEmpty is true, return 0 instead of empty map
	if returnZeroWhenEmpty && len(val) == 0 {
		if nameOverride != "" {
			metricName = nameOverride
		}
		val[metricName] = GaugeValue(0)
	}

	return val, nil
}

// CountFromValueWithLabelsFilter works like FromValueWithLabelsFilter but returns 0 instead of empty
// when no metrics match the label filter. This is useful for count/gauge metrics where absence means 0.
//
// For example, counting addresses with ready="true" should return 0 if there are no ready addresses,
// rather than returning an empty result (which would be treated as "metric not found").
func CountFromValueWithLabelsFilter(metricName string, nameOverride string, labelsFilter ...LabelsFilter) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		value, err := definition.FromRaw(metricName)(groupLabel, entityID, groups)
		if err != nil {
			// Only handle "metric not found" errors by returning 0
			// Other errors (group not found, entity not found, parsing errors) should be propagated
			if strings.Contains(err.Error(), "metric") && strings.Contains(err.Error(), "not found") {
				// For count metrics, when the underlying metric doesn't exist at all, treat it as an empty array
				// and let fetchedValuesFromRawMetricsWithLabels handle it (will return 0).
				// This handles cases where KSM >= v2.14 doesn't report individual address metrics when
				// an endpoint has 0 addresses. For example, k8s.io-minikube-hostpath exists as a Service
				// but has no backing pods, so no kube_endpoint_address metrics exist for it.
				// In KSM < v2.14, this was reported as kube_endpoint_address_available=0, but in v2.14+
				// the metric is simply absent. By treating it as an empty array, we get consistent behavior.
				return fetchedValuesFromRawMetricsWithLabels(metricName, nameOverride, []Metric{}, true, labelsFilter...)
			}
			// For other errors (group/entity not found, parsing errors, etc.), propagate them
			return nil, err
		}

		switch m := value.(type) {
		case Metric:
			return m.Value, nil
		case []Metric:
			return fetchedValuesFromRawMetricsWithLabels(metricName, nameOverride, m, true, labelsFilter...)
		}
		return nil, fmt.Errorf(
			"incompatible metric type for %s. Expected: Metric or []Metric. Got: %T",
			metricName,
			value,
		)
	}
}

// hasMatchingLabels checks if a metric has any matching label given a list of labels filter funcs.
func hasMatchingLabels(metric Metric, labelsFilter ...LabelsFilter) bool {
	labels := copyMapLabels(metric.Labels)
	for _, filter := range labelsFilter {
		labels = filter(labels)
	}

	return len(labels) != 0
}

// getMetricValue return the value of a given metric taking into account the aggregated and the metric value itself.
func getMetricValue(metricName string, aggregatedValue definition.FetchedValue, metric Metric) (definition.FetchedValue, error) {
	var value definition.FetchedValue

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
		value = aggregatedCounter + metric.Value.(CounterValue)
	case GaugeValue:
		aggregatedCounter, ok := aggregatedValue.(GaugeValue)
		if !ok {
			return nil, fmt.Errorf(
				"incompatible metric type for %s aggregation. Expected: GaugeValue. Got: %T",
				metricName,
				metric.Value,
			)
		}
		value = aggregatedCounter + metric.Value.(GaugeValue)
	}

	return value, nil
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

	return metricKey, value
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
	return metricKey, value, err
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
			return nil, fmt.Errorf("cannot retrieve the entity ID of metrics to inherit value from, got error: %w", err)
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
// are formatted from "<prefix>_" to "<prefix>.".
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
			"cannot retrieve the entity ID of metrics to inherit labels from, got error: %w",
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
	case "node", "namespace", "persistentvolume":
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

func copyMapLabels(labels Labels) Labels {
	targetMap := make(Labels)
	for key, value := range labels {
		targetMap[key] = value
	}
	return targetMap
}
