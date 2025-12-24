//nolint:paralleltest // Some tests intentionally do not use t.Parallel or use it in subtests only.
package prometheus

import (
	"errors"
	"fmt"
	"math"
	"testing"

	model "github.com/prometheus/client_model/go"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var mFamily = []MetricFamily{
	{
		Name: "kube_pod_start_time",
		Metrics: []Metric{
			{
				Value: GaugeValue(1507117436),
				Labels: map[string]string{
					"namespace": "kube-system",
					"pod":       "fluentd-elasticsearch-jnqb7",
				},
			},
			{
				Value: GaugeValue(1510579152),
				Labels: map[string]string{
					"namespace": "kube-system",
					"pod":       "newrelic-infra-monitoring-cglrn",
				},
			},
		},
	},
	{
		Name: "kube_pod_info",
		Metrics: []Metric{
			{
				Value: GaugeValue(1),
				Labels: map[string]string{
					"created_by_kind": "DaemonSet",
					"created_by_name": "fluentd-elasticsearch",
					"namespace":       "kube-system",
					"node":            "minikube",
					"pod":             "fluentd-elasticsearch-jnqb7",
				},
			},
			{
				Value: GaugeValue(1),
				Labels: map[string]string{
					"created_by_kind": "DaemonSet",
					"created_by_name": "newrelic-infra-monitoring",
					"namespace":       "kube-system",
					"node":            "minikube",
					"pod":             "newrelic-infra-monitoring-cglrn",
				},
			},
		},
	},
	{
		Name: "kube_pod_labels",
		Metrics: []Metric{
			{
				Value: GaugeValue(1),
				Labels: map[string]string{
					"label_app":                      "newrelic-infra-monitoring",
					"label_controller_revision_hash": "1758702902",
					"label_pod_template_generation":  "1",
					"namespace":                      "kube-system",
					"pod":                            "newrelic-infra-monitoring-cglrn",
				},
			},
			{
				Value: GaugeValue(1),
				Labels: map[string]string{
					"label_name":                     "fluentd-elasticsearch",
					"label_controller_revision_hash": "3534845553",
					"label_pod_template_generation":  "1",
					"namespace":                      "kube-system",
					"pod":                            "fluentd-elasticsearch-jnqb7",
				},
			},
		},
	},
}

var spec = []definition.Spec{
	{
		Name:      "podStartTime",
		ValueFunc: FromValue("kube_pod_start_time"),
		Type:      metric.GAUGE,
	},
	{
		Name:      "podInfo.namespace",
		ValueFunc: FromLabelValue("kube_pod_info", "namespace"),
		Type:      metric.ATTRIBUTE,
	},
	{
		Name:      "podInfo.pod",
		ValueFunc: FromLabelValue("kube_pod_info", "pod"),
		Type:      metric.ATTRIBUTE,
	},
}

var containersSpec = definition.SpecGroups{
	"container": definition.SpecGroup{
		Specs: []definition.Spec{
			{
				Name:      "container",
				ValueFunc: FromLabelValue("kube_pod_container_info", "container"),
				Type:      metric.ATTRIBUTE,
			},
			{
				Name:      "image",
				ValueFunc: FromLabelValue("kube_pod_container_info", "image"),
				Type:      metric.ATTRIBUTE,
			},
			{
				Name:      "namespace",
				ValueFunc: FromLabelValue("kube_pod_container_info", "namespace"),
				Type:      metric.ATTRIBUTE,
			},
			{
				Name:      "pod",
				ValueFunc: FromLabelValue("kube_pod_container_info", "pod"),
				Type:      metric.ATTRIBUTE,
			},
		},
	},
}

var specs = definition.SpecGroups{
	"pod": definition.SpecGroup{
		Specs: spec,
	},
}

var metricFamilyContainersWithTheSameName = []MetricFamily{
	{
		Name: "kube_pod_container_info",
		Metrics: []Metric{
			{
				Value: GaugeValue(1),
				Labels: map[string]string{
					"container": "kube-state-metrics",
					"image":     "gcr.io/google_containers/kube-state-metrics:v1.1.0",
					"namespace": "kube-system",
					"pod":       "newrelic-infra-monitoring-3bxnh",
				},
			},
			{
				Value: GaugeValue(1),
				Labels: map[string]string{
					"container": "kube-state-metrics",
					"image":     "gcr.io/google_containers/kube-state-metrics:v1.1.0",
					"namespace": "kube-system",
					"pod":       "fluentd-elasticsearch-jnqb7",
				},
			},
		},
	},
}

var rawGroups = definition.RawGroups{
	"pod": {
		"fluentd-elasticsearch-jnqb7": definition.RawMetrics{
			"kube_pod_start_time": Metric{
				Value: GaugeValue(1507117436),
				Labels: map[string]string{
					"namespace": "kube-system",
					"pod":       "fluentd-elasticsearch-jnqb7",
				},
			},
			"kube_pod_info": Metric{
				Value: GaugeValue(1),
				Labels: map[string]string{
					"created_by_kind": "ReplicaSet",
					"created_by_name": "fluentd-elasticsearch-fafnoa",
					"namespace":       "kube-system",
					"node":            "minikube",
					"pod":             "fluentd-elasticsearch-jnqb7",
				},
			},
			"kube_pod_status_phase": Metric{
				Value: GaugeValue(1),
				Labels: map[string]string{
					"namespace": "kube-system",
					"pod":       "fluentd-elasticsearch-jnqb7",
					"phase":     "Pending",
				},
			},
			"kube_pod_status_scheduled": Metric{
				Value: GaugeValue(1),
				Labels: map[string]string{
					"namespace": "kube-system",
					"pod":       "fluentd-elasticsearch-jnqb7",
					"condition": "false",
				},
			},
		},
		"newrelic-infra-monitoring-cglrn": definition.RawMetrics{
			"kube_pod_start_time": Metric{
				Value: GaugeValue(1510579152),
				Labels: map[string]string{
					"namespace": "kube-system",
					"pod":       "newrelic-infra-monitoring-cglrn",
				},
			},
			"kube_pod_info": Metric{
				Value: GaugeValue(1),
				Labels: map[string]string{
					"created_by_kind": "DaemonSet",
					"created_by_name": "newrelic-infra-monitoring",
					"namespace":       "kube-system",
					"node":            "minikube",
					"pod":             "newrelic-infra-monitoring-cglrn",
				},
			},
		},
		"kubernetes-dashboard-77d8b98585-c8s22": definition.RawMetrics{
			"kube_pod_status_phase": Metric{
				Value: GaugeValue(1),
				Labels: map[string]string{
					"namespace": "kube-system",
					"pod":       "kubernetes-dashboard-77d8b98585-c8s22",
					"phase":     "Pending",
				},
			},
			"kube_pod_status_scheduled": Metric{
				Value: GaugeValue(1),
				Labels: map[string]string{
					"namespace": "kube-system",
					"pod":       "kubernetes-dashboard-77d8b98585-c8s22",
					"condition": "true",
				},
			},
		},
	},
}

var rawGroupsIncompatibleType = definition.RawGroups{
	"pod": {
		"fluentd-elasticsearch-jnqb7": definition.RawMetrics{
			"kube_pod_start_time": "foo",
		},
	},
}

var summarySpec = definition.SpecGroups{
	"scheduler": definition.SpecGroup{
		Specs: []definition.Spec{
			{Name: "http_request_duration_microseconds", ValueFunc: FromSummary("http_request_duration_microseconds"), Type: metric.GAUGE},
		},
	},
}

func float64Ptr(f float64) *float64 {
	return &f
}

func uint64Ptr(u uint64) *uint64 {
	return &u
}

var summaryRawGroups = definition.RawGroups{
	"scheduler": {
		"kube-scheduler-minikube": {
			"http_request_duration_microseconds": []Metric{
				{
					Labels: Labels{"l2": "v2", "l1": "v1", "handler": "prometheus"},
					Value: &model.Summary{
						SampleCount: uint64Ptr(5),
						SampleSum:   float64Ptr(45),
						Quantile: []*model.Quantile{
							{
								Quantile: float64Ptr(0.5),
								Value:    float64Ptr(42),
							},
							{
								Quantile: float64Ptr(0.9),
								Value:    float64Ptr(43),
							},
							{
								Quantile: float64Ptr(0.99),
								Value:    float64Ptr(44),
							},
						},
					},
				}, {
					Labels: Labels{"l2": "v2", "l1": "v1", "handler": "other"},
					Value: &model.Summary{
						SampleCount: uint64Ptr(5),
						SampleSum:   float64Ptr(45),
						Quantile: []*model.Quantile{
							{
								Quantile: float64Ptr(0.5),
								Value:    float64Ptr(42),
							},
							{
								Quantile: float64Ptr(0.9),
								Value:    float64Ptr(43),
							},
							{
								Quantile: float64Ptr(0.99),
								Value:    float64Ptr(44),
							},
						},
					},
				},
			},
		},
	},
}

var summaryMetricFamily = []MetricFamily{
	{
		Name: "http_request_duration_microseconds",
		Type: "SUMMARY",
		Metrics: []Metric{
			{
				Labels: Labels{"l2": "v2", "l1": "v1", "handler": "prometheus"},
				Value: &model.Summary{
					SampleCount: uint64Ptr(5),
					SampleSum:   float64Ptr(45),
					Quantile: []*model.Quantile{
						{
							Quantile: float64Ptr(0.5),
							Value:    float64Ptr(42),
						},
						{
							Quantile: float64Ptr(0.9),
							Value:    float64Ptr(43),
						},
						{
							Quantile: float64Ptr(0.99),
							Value:    float64Ptr(44),
						},
					},
				},
			},
			{
				Labels: Labels{"l2": "v2", "l1": "v1", "handler": "other"},
				Value: &model.Summary{
					SampleCount: uint64Ptr(5),
					SampleSum:   float64Ptr(45),
					Quantile: []*model.Quantile{
						{
							Quantile: float64Ptr(0.5),
							Value:    float64Ptr(42),
						},
						{
							Quantile: float64Ptr(0.9),
							Value:    float64Ptr(43),
						},
						{
							Quantile: float64Ptr(0.99),
							Value:    float64Ptr(44),
						},
					},
				},
			},
		},
	},
}

// --------------- GroupMetricsBySpec ---------------.
func TestGroupMetricsBySpec_CorrectValue(t *testing.T) {
	expectedMetricGroup := definition.RawGroups{
		"pod": {
			"kube-system_fluentd-elasticsearch-jnqb7": definition.RawMetrics{
				"kube_pod_start_time": Metric{
					Value: GaugeValue(1507117436),
					Labels: map[string]string{
						"namespace": "kube-system",
						"pod":       "fluentd-elasticsearch-jnqb7",
					},
				},
				"kube_pod_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"created_by_kind": "DaemonSet",
						"created_by_name": "fluentd-elasticsearch",
						"namespace":       "kube-system",
						"node":            "minikube",
						"pod":             "fluentd-elasticsearch-jnqb7",
					},
				},
				"kube_pod_labels": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"label_name":                     "fluentd-elasticsearch",
						"label_controller_revision_hash": "3534845553",
						"label_pod_template_generation":  "1",
						"namespace":                      "kube-system",
						"pod":                            "fluentd-elasticsearch-jnqb7",
					},
				},
			},
			"kube-system_newrelic-infra-monitoring-cglrn": definition.RawMetrics{
				"kube_pod_start_time": Metric{
					Value: GaugeValue(1510579152),
					Labels: map[string]string{
						"namespace": "kube-system",
						"pod":       "newrelic-infra-monitoring-cglrn",
					},
				},
				"kube_pod_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"created_by_kind": "DaemonSet",
						"created_by_name": "newrelic-infra-monitoring",
						"namespace":       "kube-system",
						"node":            "minikube",
						"pod":             "newrelic-infra-monitoring-cglrn",
					},
				},
				"kube_pod_labels": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"label_app":                      "newrelic-infra-monitoring",
						"label_controller_revision_hash": "1758702902",
						"label_pod_template_generation":  "1",
						"namespace":                      "kube-system",
						"pod":                            "newrelic-infra-monitoring-cglrn",
					},
				},
			},
		},
	}

	metricGroup, errs := GroupMetricsBySpec(specs, mFamily)
	assert.Empty(t, errs)
	assert.Equal(t, expectedMetricGroup, metricGroup)
}

func TestGroupMetricsBySpec_CorrectValue_ContainersWithTheSameName(t *testing.T) {
	expectedMetricGroup := definition.RawGroups{
		"container": {
			"kube-system_fluentd-elasticsearch-jnqb7_kube-state-metrics": definition.RawMetrics{
				"kube_pod_container_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"container": "kube-state-metrics",
						"image":     "gcr.io/google_containers/kube-state-metrics:v1.1.0",
						"namespace": "kube-system",
						"pod":       "fluentd-elasticsearch-jnqb7",
					},
				},
			},
			"kube-system_newrelic-infra-monitoring-3bxnh_kube-state-metrics": definition.RawMetrics{
				"kube_pod_container_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"container": "kube-state-metrics",
						"image":     "gcr.io/google_containers/kube-state-metrics:v1.1.0",
						"namespace": "kube-system",
						"pod":       "newrelic-infra-monitoring-3bxnh",
					},
				},
			},
		},
	}

	metricGroup, errs := GroupMetricsBySpec(containersSpec, metricFamilyContainersWithTheSameName)
	assert.Empty(t, errs)
	assert.Equal(t, expectedMetricGroup, metricGroup)
}

func Test_GroupMetricsBySpec_does_not_add_unrelated_entity_metrics(t *testing.T) {
	groupLabelMetricName := "groupLabelMetricName"
	groupLabelRawMetricName := "groupLabelRawMetricName"
	groupLabelEntityName := "groupLabelEntityName"

	unrelatedMetricName := "unrelatedMetricName"
	unrelatedRawMetricName := "unrelatedRawMetricName"
	unreleatedEntityName := "unreleatedEntityName"

	cases := map[string]struct {
		groupLabel          string
		unrelatedLabel      string
		unrelatedMetricName string
	}{
		"from_daemonset_to_namespace": {
			groupLabel:          "namespace",
			unrelatedLabel:      "daemonset",
			unrelatedMetricName: fmt.Sprintf("%s_%s", groupLabelEntityName, unreleatedEntityName),
		},
		"from_pod_to_namespace": {
			groupLabel:          "namespace",
			unrelatedLabel:      "pod",
			unrelatedMetricName: fmt.Sprintf("%s_%s", groupLabelEntityName, unreleatedEntityName),
		},
		"from_endpoint_to_namespace": {
			groupLabel:          "namespace",
			unrelatedLabel:      "endpoint",
			unrelatedMetricName: fmt.Sprintf("%s_%s", groupLabelEntityName, unreleatedEntityName),
		},
		"from_service_to_namespace": {
			groupLabel:          "namespace",
			unrelatedLabel:      "service",
			unrelatedMetricName: fmt.Sprintf("%s_%s", groupLabelEntityName, unreleatedEntityName),
		},
		"from_deployment_to_namespace": {
			groupLabel:          "namespace",
			unrelatedLabel:      "deployment",
			unrelatedMetricName: fmt.Sprintf("%s_%s", groupLabelEntityName, unreleatedEntityName),
		},
		"from_replicaset_to_namespace": {
			groupLabel:          "namespace",
			unrelatedLabel:      "replicaset",
			unrelatedMetricName: fmt.Sprintf("%s_%s", groupLabelEntityName, unreleatedEntityName),
		},
		"from_pod_to_node": {
			groupLabel:          "node",
			unrelatedLabel:      "pod",
			unrelatedMetricName: fmt.Sprintf("_%s", unreleatedEntityName),
		},
	}

	for caseName, c := range cases {
		c := c
		t.Run(caseName, func(t *testing.T) {
			spec := definition.SpecGroups{
				c.groupLabel: definition.SpecGroup{
					Specs: []definition.Spec{
						{
							Name:      groupLabelMetricName,
							ValueFunc: FromValue(groupLabelRawMetricName),
							Type:      metric.GAUGE,
						},
					},
				},
				c.unrelatedLabel: definition.SpecGroup{
					Specs: []definition.Spec{
						{
							Name:      unrelatedMetricName,
							ValueFunc: FromValue(unrelatedRawMetricName),
							Type:      metric.GAUGE,
						},
					},
				},
			}

			metricFamily := []MetricFamily{
				{
					Name: groupLabelRawMetricName,
					Metrics: []Metric{
						{
							Value: GaugeValue(1),
							Labels: map[string]string{
								c.groupLabel: groupLabelEntityName,
							},
						},
					},
				},
				{
					Name: unrelatedRawMetricName,
					Metrics: []Metric{
						{
							Value: GaugeValue(1),
							Labels: map[string]string{
								c.groupLabel:     groupLabelEntityName,
								c.unrelatedLabel: unreleatedEntityName,
							},
						},
					},
				},
			}

			expectedRawGroups := definition.RawGroups{
				c.groupLabel: {
					groupLabelEntityName: definition.RawMetrics{
						groupLabelRawMetricName: Metric{
							Value: GaugeValue(1),
							Labels: map[string]string{
								c.groupLabel: groupLabelEntityName,
							},
						},
					},
				},
				c.unrelatedLabel: {
					c.unrelatedMetricName: definition.RawMetrics{
						unrelatedRawMetricName: Metric{
							Value: GaugeValue(1),
							Labels: map[string]string{
								c.groupLabel:     groupLabelEntityName,
								c.unrelatedLabel: unreleatedEntityName,
							},
						},
					},
				},
			}

			metricGroup, errs := GroupMetricsBySpec(spec, metricFamily)
			assert.Empty(t, errs)
			assert.Equal(t, expectedRawGroups, metricGroup)
		})
	}
}

func TestGroupMetricsBySpec_EmptyMetricFamily(t *testing.T) {
	var emptyMetricFamily []MetricFamily

	metricGroup, errs := GroupMetricsBySpec(specs, emptyMetricFamily)
	assert.Len(t, errs, 1)
	assert.Equal(t, errors.New("no data found for pod object"), errs[0])
	assert.Empty(t, metricGroup)
}

// To preserve old behavior.
func TestGroupMetricsBySpec_returns_single_metric_with_one_metric_in_metric_family(t *testing.T) {
	groupLabel := "node"
	metricName := "metricName"
	rawMetricName := "rawMetricName"
	entityName := "entityName"

	spec := definition.SpecGroups{
		groupLabel: definition.SpecGroup{
			Specs: []definition.Spec{
				{
					Name:      metricName,
					ValueFunc: FromValue(rawMetricName),
					Type:      metric.GAUGE,
				},
			},
		},
	}

	metricFamily := []MetricFamily{
		{
			Name: rawMetricName,
			Metrics: []Metric{
				{
					Value: GaugeValue(1),
					Labels: map[string]string{
						groupLabel: entityName,
					},
				},
			},
		},
	}

	expectedRawGroups := definition.RawGroups{
		groupLabel: {
			entityName: definition.RawMetrics{
				rawMetricName: Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						groupLabel: entityName,
					},
				},
			},
		},
	}

	metricGroup, errs := GroupMetricsBySpec(spec, metricFamily)
	assert.Empty(t, errs)
	assert.Equal(t, expectedRawGroups, metricGroup)
}

// To be able to process multiple metrics of the same type, like 'kube_node_status_condition'.
func TestGroupMetricsBySpec_does_not_override_metric_when_there_is_more_than_one_in_metric_family(t *testing.T) {
	groupLabel := "node"
	metricName := "metricName"
	rawMetricName := "rawMetricName"
	entityName := "entityName"

	spec := definition.SpecGroups{
		groupLabel: definition.SpecGroup{
			Specs: []definition.Spec{
				{
					Name:      metricName,
					ValueFunc: FromValue(rawMetricName),
					Type:      metric.GAUGE,
				},
			},
		},
	}

	metricFamily := []MetricFamily{
		{
			Name: rawMetricName,
			Metrics: []Metric{
				{
					Value: GaugeValue(1),
					Labels: map[string]string{
						groupLabel:  entityName,
						"condition": "DiskPressure",
					},
				},
				{
					Value: GaugeValue(1),
					Labels: map[string]string{
						groupLabel:  entityName,
						"condition": "MemoryPressure",
					},
				},
			},
		},
	}

	expectedRawGroups := definition.RawGroups{
		groupLabel: {
			entityName: definition.RawMetrics{
				rawMetricName: []Metric{
					{
						Value: GaugeValue(1),
						Labels: map[string]string{
							groupLabel:  entityName,
							"condition": "DiskPressure",
						},
					},
					{
						Value: GaugeValue(1),
						Labels: map[string]string{
							groupLabel:  entityName,
							"condition": "MemoryPressure",
						},
					},
				},
			},
		},
	}

	metricGroup, errs := GroupMetricsBySpec(spec, metricFamily)
	assert.Empty(t, errs)
	assert.Equal(t, expectedRawGroups, metricGroup)
}

func TestGroupEntityMetricsBySpec_CorrectValue(t *testing.T) {
	metricGroup, errs := GroupEntityMetricsBySpec(
		summarySpec,
		summaryMetricFamily,
		"kube-scheduler-minikube",
	)
	assert.Empty(t, errs)
	assert.Equal(t, summaryRawGroups, metricGroup)
}

func TestGroupEntityMetricsBySpec_NoMatch(t *testing.T) {
	var emptyMetricFamily []MetricFamily

	metricGroup, errs := GroupEntityMetricsBySpec(
		summarySpec,
		emptyMetricFamily,
		"kube-scheduler-minikube",
	)
	assert.Len(t, errs, 1)
	assert.Equal(t, errors.New("no data found for scheduler object"), errs[0])
	assert.Empty(t, metricGroup)
}

func TestFetchFuncs_CorrectValue(t *testing.T) {
	testCases := []struct {
		name                 string
		rawGroups            definition.RawGroups
		expectedFetchedValue definition.FetchedValues
		fetchFunc            definition.FetchFunc
	}{
		{
			name: "FromValue correct value",
			rawGroups: definition.RawGroups{
				"scheduler": {
					"kube-scheduler-minikube": {
						"leader_election_master_status": []Metric{
							{
								Labels: Labels{"name": "kube-scheduler"},
								Value:  GaugeValue(1),
							},
						},
					},
				},
			},
			fetchFunc: FromValue("leader_election_master_status", IgnoreLabelsFilter("name")),
			expectedFetchedValue: definition.FetchedValues{
				"leader_election_master_status": GaugeValue(1),
			},
		},
		{
			name: "FromValueOverriddenName sets the correct name",
			rawGroups: definition.RawGroups{
				"scheduler": {
					"kube-scheduler-minikube": {
						"http_request_count": []Metric{
							{
								Labels: Labels{"verb": "GET"},
								Value:  GaugeValue(1),
							},
							{
								Labels: Labels{"verb": "POST"},
								Value:  GaugeValue(9),
							},
						},
					},
				},
			},
			fetchFunc: FromValueWithOverriddenName("http_request_count", "my_custom_request_count"),
			expectedFetchedValue: definition.FetchedValues{
				"my_custom_request_count_verb_GET":  GaugeValue(1),
				"my_custom_request_count_verb_POST": GaugeValue(9),
			},
		},
		{
			name: "FromValue correct multiple values",
			rawGroups: definition.RawGroups{
				"scheduler": {
					"kube-scheduler-minikube": {
						"leader_election_master_status": []Metric{
							{
								Labels: Labels{"name": "kube-scheduler", "l": "v1"},
								Value:  GaugeValue(1),
							},
							{
								Labels: Labels{"name": "kube-scheduler", "l": "v2"},
								Value:  GaugeValue(0),
							},
						},
					},
				},
			},
			fetchFunc: FromValue("leader_election_master_status", IgnoreLabelsFilter("name")),
			expectedFetchedValue: definition.FetchedValues{
				"leader_election_master_status_l_v1": GaugeValue(1),
				"leader_election_master_status_l_v2": GaugeValue(0),
			},
		},
		{
			name: "FromValue correct aggregated values",
			rawGroups: definition.RawGroups{
				"scheduler": {
					"kube-scheduler-minikube": {
						"leader_election_master_status": []Metric{
							{
								Labels: Labels{"name": "kube-scheduler", "l": "v1"},
								Value:  CounterValue(1),
							},
							{
								Labels: Labels{"name": "kube-scheduler", "l": "v2"},
								Value:  CounterValue(2),
							},
							{
								Labels: Labels{"name": "kube-scheduler-02", "l": "v1"},
								Value:  CounterValue(3),
							},
							{
								Labels: Labels{"name": "kube-scheduler-02", "l": "v2"},
								Value:  CounterValue(4),
							},
						},
					},
				},
			},
			fetchFunc: FromValue("leader_election_master_status", IncludeOnlyLabelsFilter("name")),
			expectedFetchedValue: definition.FetchedValues{
				"leader_election_master_status_name_kube-scheduler":    CounterValue(3),
				"leader_election_master_status_name_kube-scheduler-02": CounterValue(7),
			},
		},
		{
			name: "FromValueWithLabelsFilter skip aggregates value when no filter",
			rawGroups: definition.RawGroups{
				"scheduler": {
					"kube-scheduler-minikube": {
						"scheduler_pending_pods": []Metric{
							{
								Labels: Labels{"queue": "active"},
								Value:  CounterValue(1),
							},
						},
					},
				},
			},
			fetchFunc: FromValueWithLabelsFilter(
				"scheduler_pending_pods",
				"",
				IncludeOnlyWhenLabelMatchFilter(nil),
			),
			expectedFetchedValue: definition.FetchedValues{},
		},
		{
			name: "FromValueWithLabelsFilter correct aggregates value with single filter",
			rawGroups: definition.RawGroups{
				"scheduler": {
					"kube-scheduler-minikube": {
						"scheduler_pending_pods": []Metric{
							{
								Labels: Labels{"queue": "active"},
								Value:  CounterValue(1),
							},
							{
								Labels: Labels{"queue": "backoff"},
								Value:  CounterValue(2),
							},
							{
								Labels: Labels{"queue": "active"},
								Value:  CounterValue(4),
							},
						},
					},
				},
			},
			fetchFunc: FromValueWithLabelsFilter(
				"scheduler_pending_pods",
				"",
				IncludeOnlyWhenLabelMatchFilter(map[string]string{"queue": "active"}),
			),
			expectedFetchedValue: definition.FetchedValues{
				"scheduler_pending_pods": CounterValue(5),
			},
		},
		{
			name: "FromValueWithLabelsFilter correct aggregates values with multiple filters",
			rawGroups: definition.RawGroups{
				"scheduler": {
					"kube-scheduler-minikube": {
						"scheduler_pending_pods": []Metric{
							{
								Labels: Labels{"queue": "active"},
								Value:  CounterValue(1),
							},
							{
								Labels: Labels{"queue": "active", "l": "v2"},
								Value:  CounterValue(2),
							},
							{
								Labels: Labels{"queue": "backoff"},
								Value:  CounterValue(1),
							},
						},
					},
				},
			},
			fetchFunc: FromValueWithLabelsFilter(
				"scheduler_pending_pods",
				"",
				IncludeOnlyWhenLabelMatchFilter(map[string]string{
					"queue": "active",
					"l":     "v2",
				}),
			),
			expectedFetchedValue: definition.FetchedValues{
				"scheduler_pending_pods": CounterValue(3),
			},
		},
		{
			name: "FromSummary correct values with NaN and Infinite discarded",
			rawGroups: definition.RawGroups{
				"scheduler": {
					"kube-scheduler-minikube": {
						"http_request_duration_microseconds": []Metric{
							{
								Labels: Labels{"l2": "v2", "l1": "v1", "handler": "prometheus"},
								Value: &model.Summary{
									SampleCount: uint64Ptr(5),
									SampleSum:   float64Ptr(math.Inf(1)),
									Quantile: []*model.Quantile{
										{
											Quantile: float64Ptr(0.5),
											Value:    float64Ptr(math.NaN()),
										},
										{
											Quantile: float64Ptr(0.9),
											Value:    float64Ptr(math.NaN()),
										},
										{
											Quantile: float64Ptr(0.99),
											Value:    float64Ptr(44),
										},
									},
								},
							},
						},
					},
				},
			},
			fetchFunc: FromSummary("http_request_duration_microseconds"),
			expectedFetchedValue: definition.FetchedValues{
				"http_request_duration_microseconds_handler_prometheus_l1_v1_l2_v2_count":         uint64(5),
				"http_request_duration_microseconds_handler_prometheus_l1_v1_l2_v2_quantile_0.99": float64(44),
			},
		},
		{
			name:      "FromSummary correct value",
			rawGroups: summaryRawGroups,
			fetchFunc: FromSummary("http_request_duration_microseconds"),
			expectedFetchedValue: definition.FetchedValues{
				"http_request_duration_microseconds_handler_prometheus_l1_v1_l2_v2_count":         uint64(5),
				"http_request_duration_microseconds_handler_prometheus_l1_v1_l2_v2_quantile_0.5":  float64(42),
				"http_request_duration_microseconds_handler_prometheus_l1_v1_l2_v2_quantile_0.9":  float64(43),
				"http_request_duration_microseconds_handler_prometheus_l1_v1_l2_v2_quantile_0.99": float64(44),
				"http_request_duration_microseconds_handler_prometheus_l1_v1_l2_v2_sum":           float64(45),
				"http_request_duration_microseconds_handler_other_l1_v1_l2_v2_count":              uint64(5),
				"http_request_duration_microseconds_handler_other_l1_v1_l2_v2_quantile_0.5":       float64(42),
				"http_request_duration_microseconds_handler_other_l1_v1_l2_v2_quantile_0.9":       float64(43),
				"http_request_duration_microseconds_handler_other_l1_v1_l2_v2_quantile_0.99":      float64(44),
				"http_request_duration_microseconds_handler_other_l1_v1_l2_v2_sum":                float64(45),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fetchedValue, err := testCase.fetchFunc(
				"scheduler",
				"kube-scheduler-minikube",
				testCase.rawGroups,
			)
			assert.Equal(t, testCase.expectedFetchedValue, fetchedValue)
			assert.NoError(t, err)
		})
	}
}

func TestFetchFunc_RawMetricNotFound(t *testing.T) {
	testCases := []struct {
		name                 string
		rawGroups            definition.RawGroups
		expectedFetchedValue definition.FetchedValues
		fetchFunc            definition.FetchFunc
	}{
		{
			name: "FromValue",
			rawGroups: definition.RawGroups{
				"scheduler": {
					"kube-scheduler-minikube": {
						"leader_election_master_status": []Metric{
							{
								Labels: Labels{"name": "kube-scheduler", "l": "v1"},
								Value:  GaugeValue(1),
							},
						},
					},
				},
			},
			fetchFunc: FromValue("nope"),
		},
		{
			name:      "FromSummary",
			rawGroups: summaryRawGroups,
			fetchFunc: FromSummary("nope"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fetchedValue, err := testCase.fetchFunc(
				"scheduler",
				"kube-scheduler-minikube",
				testCase.rawGroups,
			)
			assert.Nil(t, fetchedValue)
			assert.EqualError(t, err, "metric \"nope\" not found")
		})
	}
}

func TestFetchFunc_IncompatibleType(t *testing.T) {
	testCases := []struct {
		name                 string
		rawGroups            definition.RawGroups
		expectedFetchedValue definition.FetchedValues
		fetchFunc            definition.FetchFunc
		expectedType         string
		actualType           string
		key                  string
	}{
		{
			name: "FromValue",
			rawGroups: definition.RawGroups{
				"scheduler": {
					"kube-scheduler-minikube": {
						"leader_election_master_status": GaugeValue(1),
					},
				},
			},
			fetchFunc:    FromValue("leader_election_master_status"),
			expectedType: "Metric or []Metric",
			actualType:   "prometheus.GaugeValue",
			key:          "leader_election_master_status",
		},
		{
			name: "FromSummaryNo[]Metric",
			rawGroups: definition.RawGroups{
				"scheduler": {
					"kube-scheduler-minikube": {
						"http_request_duration_microseconds": GaugeValue(1),
					},
				},
			},
			fetchFunc:    FromSummary("http_request_duration_microseconds"),
			expectedType: "[]Metric",
			actualType:   "prometheus.GaugeValue",
			key:          "http_request_duration_microseconds",
		},
		{
			name: "FromSummaryNoSummary",
			rawGroups: definition.RawGroups{
				"scheduler": {
					"kube-scheduler-minikube": {
						"http_request_duration_microseconds": []Metric{
							{
								Labels: Labels{"l2": "v2", "l1": "v1", "handler": "prometheus"},
								Value:  GaugeValue(1),
							},
						},
					},
				},
			},
			fetchFunc:    FromSummary("http_request_duration_microseconds"),
			expectedType: "Summary",
			actualType:   "prometheus.GaugeValue",
			key:          "http_request_duration_microseconds",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fetchedValue, err := testCase.fetchFunc(
				"scheduler",
				"kube-scheduler-minikube",
				testCase.rawGroups,
			)
			assert.Nil(t, fetchedValue)
			assert.EqualError(
				t,
				err,
				fmt.Sprintf(
					"incompatible metric type for %s. Expected: %s. Got: %s",
					testCase.key,
					testCase.expectedType,
					testCase.actualType,
				),
			)
		})
	}
}

// --------------- FromValue ---------------.
func TestFromRawValue_CorrectValue(t *testing.T) {
	expectedFetchedValue := GaugeValue(1507117436)

	fetchedValue, err := FromValue("kube_pod_start_time")("pod", "fluentd-elasticsearch-jnqb7", rawGroups)
	assert.Equal(t, expectedFetchedValue, fetchedValue)
	assert.NoError(t, err)
}

func TestFromRawValue_RawMetricNotFound(t *testing.T) {
	fetchedValue, err := FromValue("foo")("pod", "fluentd-elasticsearch-jnqb7", rawGroups)
	assert.Nil(t, fetchedValue)
	assert.EqualError(t, err, "metric \"foo\" not found")
}

func TestFromRawValue_IncompatibleType(t *testing.T) {
	fetchedValue, err := FromValue("kube_pod_start_time")("pod", "fluentd-elasticsearch-jnqb7", rawGroupsIncompatibleType)
	assert.Nil(t, fetchedValue)
	assert.EqualError(
		t,
		err,
		fmt.Sprintf(
			"incompatible metric type for %s. Expected: Metric or []Metric. Got: string",
			"kube_pod_start_time",
		),
	)
}

// --------------- FromLabelValue ---------------.
func TestFromRawLabelValue_CorrectValue(t *testing.T) {
	expectedFetchedValue := "kube-system"

	fetchedValue, err := FromLabelValue("kube_pod_start_time", "namespace")("pod", "fluentd-elasticsearch-jnqb7", rawGroups)
	assert.Equal(t, expectedFetchedValue, fetchedValue)
	assert.NoError(t, err)
}

func TestFromRawLabelValue_RawMetricNotFound(t *testing.T) {
	fetchedValue, err := FromLabelValue("foo", "namespace")("pod", "fluentd-elasticsearch-jnqb7", rawGroups)
	assert.Nil(t, fetchedValue)
	assert.EqualError(t, err, "metric \"foo\" not found")
}

func TestFromRawLabelValue_IncompatibleType(t *testing.T) {
	fetchedValue, err := FromLabelValue("kube_pod_start_time", "namespace")("pod", "fluentd-elasticsearch-jnqb7", rawGroupsIncompatibleType)
	assert.Nil(t, fetchedValue)
	assert.Contains(t, err.Error(), "incompatible metric type")
}

func TestFromRawLabelValue_LabelNotFoundInRawMetric(t *testing.T) {
	fetchedValue, err := FromLabelValue("kube_pod_start_time", "foo")("pod", "fluentd-elasticsearch-jnqb7", rawGroups)
	assert.Nil(t, fetchedValue)
	assert.EqualError(t, err, "label \"foo\" not found on metric \"kube_pod_start_time\": label not found on metric")
}

// --------------- FromLabelValueEntityTypeGenerator -------------.
func TestFromLabelValueEntityTypeGenerator_CorrectValueNamespace(t *testing.T) {
	raw := definition.RawGroups{
		"namespace": {
			"kube-system": definition.RawMetrics{},
		},
	}

	expectedValue := "k8s:clusterName:namespace"

	generatedValue, err := FromLabelValueEntityTypeGenerator("kube_namespace_labels")("namespace", "kube-system", raw, "clusterName")
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, generatedValue)
}

func TestFromLabelValueEntityTypeGenerator_CorrectValueReplicaset(t *testing.T) {
	raw := definition.RawGroups{
		"replicaset": {
			"kube-state-metrics-4044341274": definition.RawMetrics{
				"kube_replicaset_created": Metric{
					Value: GaugeValue(1507117436),
					Labels: map[string]string{
						"replicaset": "kube-state-metrics-4044341274",
						"namespace":  "kube-system",
					},
				},
			},
		},
	}
	expectedValue := "k8s:clusterName:kube-system:replicaset"

	generatedValue, err := FromLabelValueEntityTypeGenerator("kube_replicaset_created")("replicaset", "kube-state-metrics-4044341274", raw, "clusterName")
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, generatedValue)
}

func TestFromLabelValueEntityTypeGenerator_CorrectValueContainer(t *testing.T) {
	raw := definition.RawGroups{
		"container": {
			"kube-system_fluentd-elasticsearch-jnqb7_kube-state-metrics": definition.RawMetrics{
				"kube_pod_container_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"container": "kube-state-metrics",
						"image":     "gcr.io/google_containers/kube-state-metrics:v1.1.0",
						"namespace": "kube-system",
						"pod":       "fluentd-elasticsearch-jnqb7",
					},
				},
			},
		},
	}
	expectedValue := "k8s:clusterName:kube-system:fluentd-elasticsearch-jnqb7:container"

	generatedValue, err := FromLabelValueEntityTypeGenerator("kube_pod_container_info")("container", "kube-system_fluentd-elasticsearch-jnqb7_kube-state-metrics", raw, "clusterName")
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, generatedValue)
}

func TestFromLabelValueEntityTypeGenerator_NotFound(t *testing.T) {
	raw := definition.RawGroups{
		"replicaset": {
			"kube-state-metrics-4044341274": definition.RawMetrics{
				"kube_replicaset_created": Metric{
					Value: GaugeValue(1507117436),
					Labels: map[string]string{
						"replicaset": "kube-state-metrics-4044341274",
					},
				},
			},
		},
	}

	generatedValue, err := FromLabelValueEntityTypeGenerator("kube_replicaset_created")("replicaset", "kube-state-metrics-4044341274", raw, "clusterName")
	assert.EqualError(t, err, "cannot fetch label \"namespace\" for metric \"kube_replicaset_created\": label \"namespace\" not found on metric \"kube_replicaset_created\": label not found on metric")
	assert.Equal(t, "", generatedValue)
}

func TestFromLabelValueEntityTypeGenerator_EmptyPodNameForContainer(t *testing.T) {
	raw := definition.RawGroups{
		"container": {
			"kube-system_fluentd-elasticsearch-jnqb7_kube-state-metrics": definition.RawMetrics{
				"kube_pod_container_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"container": "kube-state-metrics",
						"image":     "gcr.io/google_containers/kube-state-metrics:v1.1.0",
						"namespace": "kube-system",
						"pod":       "",
					},
				},
			},
		},
	}

	generatedValue, err := FromLabelValueEntityTypeGenerator("kube_pod_container_info")("container", "kube-system_fluentd-elasticsearch-jnqb7_kube-state-metrics", raw, "clusterName")
	assert.ErrorIs(t, err, ErrUnexpectedEmptyLabels)
	assert.Equal(t, "", generatedValue)
}

func TestFromLabelValueEntityTypeGenerator_EmptyNamespace(t *testing.T) {
	raw := definition.RawGroups{
		"pod": {
			"kube-system_fluentd-elasticsearch-jnqb7": definition.RawMetrics{
				"kube_pod_start_time": Metric{
					Value: GaugeValue(1507117436),
					Labels: map[string]string{
						"namespace": "",
						"pod":       "fluentd-elasticsearch-jnqb7",
					},
				},
			},
		},
	}
	generatedValue, err := FromLabelValueEntityTypeGenerator("kube_pod_start_time")("pod", "kube-system_fluentd-elasticsearch-jnqb7", raw, "clusterName")
	assert.ErrorIs(t, err, ErrUnexpectedEmptyLabels)
	assert.Equal(t, "", generatedValue)
}

// --------------- FromLabelValueEntityIDGenerator ---------------.
func TestFromLabelValueEntityIDGenerator(t *testing.T) {
	expectedFetchedValue := "fluentd-elasticsearch-jnqb7"

	fetchedValue, err := FromLabelValueEntityIDGenerator("kube_pod_info", "pod")("pod", "fluentd-elasticsearch-jnqb7", rawGroups)
	assert.NoError(t, err)
	assert.Equal(t, expectedFetchedValue, fetchedValue)
}

func TestFromLabelValueEntityIDGenerator_NotFound(t *testing.T) {
	fetchedValue, err := FromLabelValueEntityIDGenerator("non-existent-metric-key", "pod")("pod", "fluentd-elasticsearch-jnqb7", rawGroups)
	assert.Empty(t, fetchedValue)
	assert.EqualError(t, err, "cannot fetch label \"pod\" for metric \"non-existent-metric-key\": metric \"non-existent-metric-key\" not found")
}

// --------------- FromLabelsValueEntityIDGeneratorForPendingPods ---------------.
func TestFromLabelsValueEntityIDGeneratorForPendingPods(t *testing.T) {
	expectedFetchedValue := "fluentd-elasticsearch-jnqb7"

	fetchedValue, err := FromLabelsValueEntityIDGeneratorForPendingPods()("pod", "fluentd-elasticsearch-jnqb7", rawGroups)
	assert.NoError(t, err)
	assert.Equal(t, expectedFetchedValue, fetchedValue)
}

func TestFromLabelsValueEntityIDGeneratorForPendingPods_ErrorScheduledAsTrue(t *testing.T) {
	fetchedValue, err := FromLabelsValueEntityIDGeneratorForPendingPods()("pod", "kubernetes-dashboard-77d8b98585-c8s22", rawGroups)
	assert.Empty(t, fetchedValue)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "ignoring pending pod")
}

// --------------- InheritSpecificLabelValuesFrom ---------------

func TestInheritSpecificLabelValuesFrom(t *testing.T) {
	containerRawEntityID := "kube-system_kube-addon-manager-minikube_kube-addon-manager"
	raw := definition.RawGroups{
		"pod": {
			"kube-system_kube-addon-manager-minikube": definition.RawMetrics{
				"kube_pod_info": Metric{
					Value: GaugeValue(1507117436),
					Labels: map[string]string{
						"pod":       "kube-addon-manager-minikube",
						"pod_ip":    "172.31.248.38",
						"namespace": "kube-system",
					},
				},
			},
		},
		"container": {
			containerRawEntityID: definition.RawMetrics{
				"kube_pod_container_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"pod":          "kube-addon-manager-minikube",
						"container_id": "docker://441e4dacbcfb2f012f2221d0f3768552ea1ccb53454da42b7b3eeaf17bbd240a",
						"namespace":    "kube-system",
					},
				},
			},
		},
	}

	fetchedValue, err := InheritSpecificLabelValuesFrom("pod", "kube_pod_info", map[string]string{"inherited-pod_ip": "pod_ip"})("container", containerRawEntityID, raw)
	assert.NoError(t, err)

	expectedValue := definition.FetchedValues{"inherited-pod_ip": "172.31.248.38"}
	assert.Equal(t, expectedValue, fetchedValue)
}

func TestInheritSpecificLabelsFrom_Namespace(t *testing.T) {
	podRawEntityID := "kube-addon-manager-minikube"
	raw := definition.RawGroups{
		"namespace": {
			"kube-system": definition.RawMetrics{
				"kube_namespace_labels": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"namespace": "kube-system",
					},
				},
			},
		},
		"pod": {
			"kube-addon-manager-minikube": definition.RawMetrics{
				"kube_pod_info": Metric{
					Value: GaugeValue(1507117436),
					Labels: map[string]string{
						"pod":       "kube-addon-manager-minikube",
						"pod_ip":    "172.31.248.38",
						"namespace": "kube-system",
					},
				},
			},
		},
	}

	fetchedValue, err := InheritSpecificLabelValuesFrom("namespace", "kube_namespace_labels", map[string]string{"inherited-namespace": "namespace"})("pod", podRawEntityID, raw)
	assert.NoError(t, err)

	expectedValue := definition.FetchedValues{"inherited-namespace": "kube-system"}
	assert.Equal(t, expectedValue, fetchedValue)
}

func TestInheritSpecificLabelValuesFrom_RelatedMetricNotFound(t *testing.T) {
	containerRawEntityID := "kube-system_kube-addon-manager-minikube_kube-addon-manager"
	raw := definition.RawGroups{
		"pod": {},
		"container": {
			containerRawEntityID: definition.RawMetrics{
				"kube_pod_container_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"pod":       "kube-addon-manager-minikube",
						"namespace": "kube-system",
					},
				},
			},
		},
	}

	expectedPodRawEntityID := "kube-system_kube-addon-manager-minikube"
	fetchedValue, err := InheritSpecificLabelValuesFrom("pod", "non_existent_metric_key", map[string]string{"inherited-pod_ip": "pod_ip"})("container", containerRawEntityID, raw)
	assert.EqualError(t, err, fmt.Sprintf("related metric not found. Metric: non_existent_metric_key pod:%v", expectedPodRawEntityID))
	assert.Empty(t, fetchedValue)
}

func TestInheritSpecificLabelValuesFrom_NamespaceNotFound(t *testing.T) {
	containerRawEntityID := "kube-system_kube-addon-manager-minikube_kube-addon-manager"
	raw := definition.RawGroups{
		"pod": {
			"kube-addon-manager-minikube": definition.RawMetrics{
				"kube_pod_info": Metric{
					Value: GaugeValue(1507117436),
					Labels: map[string]string{
						"pod":       "kube-addon-manager-minikube",
						"pod_ip":    "172.31.248.38",
						"namespace": "kube-system",
					},
				},
			},
		},
		"container": {
			containerRawEntityID: definition.RawMetrics{
				"kube_pod_container_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"pod": "kube-addon-manager-minikube",
					},
				},
			},
		},
	}

	fetchedValue, err := InheritSpecificLabelValuesFrom("pod", "kube_pod_info", map[string]string{"inherited-pod_ip": "pod_ip"})("container", containerRawEntityID, raw)
	assert.EqualError(t, err, "cannot retrieve the entity ID of metrics to inherit value from, got error: metric with the labels [namespace pod] not found")
	assert.Empty(t, fetchedValue)
}

func TestInheritSpecificLabelValuesFrom_GroupNotFound(t *testing.T) {
	incorrectContainerRawEntityID := "non-existing-ID"
	raw := definition.RawGroups{
		"pod": {
			"kube-addon-manager-minikube": definition.RawMetrics{
				"kube_pod_info": Metric{
					Value: GaugeValue(1507117436),
					Labels: map[string]string{
						"pod":       "kube-addon-manager-minikube",
						"pod_ip":    "172.31.248.38",
						"namespace": "kube-system",
					},
				},
			},
		},
		"container": {
			"kube-addon-manager-minikube_kube-system": definition.RawMetrics{
				"kube_pod_container_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"pod":          "kube-addon-manager-minikube",
						"container_id": "docker://441e4dacbcfb2f012f2221d0f3768552ea1ccb53454da42b7b3eeaf17bbd240a",
						"namespace":    "kube-system",
					},
				},
			},
		},
	}

	fetchedValue, err := InheritSpecificLabelValuesFrom("pod", "kube_pod_info", map[string]string{"inherited-pod_ip": "pod_ip"})("container", incorrectContainerRawEntityID, raw)
	assert.EqualError(t, err, "cannot retrieve the entity ID of metrics to inherit value from, got error: metrics not found for container with entity ID: non-existing-ID")
	assert.Empty(t, fetchedValue)
}

// --------------- InheritAllSelectorsFrom ---------------.
func TestInheritAllSelectorsFrom(t *testing.T) {
	serviceRawEntityID := "kube-system_tiller-deploy"
	raw := definition.RawGroups{
		"service": {
			serviceRawEntityID: definition.RawMetrics{
				"apiserver_kube_service_spec_selectors": Metric{
					Value: nil,
					Labels: map[string]string{
						"selector_app":          "tiller",
						"selector_awesome_team": "fsi",
					},
				},
				"kube_service_info": Metric{
					Value: nil,
					Labels: map[string]string{
						"namespace": "kube-system",
						"service":   "tiller-deploy",
					},
				},
			},
		},
	}

	fetchedValue, err := InheritAllSelectorsFrom("service", "apiserver_kube_service_spec_selectors")("service", serviceRawEntityID, raw)
	require.NoError(t, err)

	expectedValue := definition.FetchedValues{
		"selector.app":          "tiller",
		"selector.awesome_team": "fsi",
	}
	assert.Equal(t, expectedValue, fetchedValue)
}

func TestInheritAllSelectorsFrom_ErrorOnOnlyOneMetricWithoutNamespaceAndServiceLabel(t *testing.T) {
	serviceRawEntityID := "kube-system_tiller-deploy"
	raw := definition.RawGroups{
		"service": {
			serviceRawEntityID: definition.RawMetrics{
				"apiserver_kube_service_spec_selectors": Metric{
					Value: nil,
					Labels: map[string]string{
						"selector_app":          "tiller",
						"selector_awesome_team": "fsi",
					},
				},
			},
		},
	}

	_, err := InheritAllSelectorsFrom("service", "apiserver_kube_service_spec_selectors")("service", serviceRawEntityID, raw)
	errorMsg := "cannot retrieve the entity ID of metrics to inherit labels from, got error: metric with the labels [namespace service] not found"
	assert.EqualError(t, err, errorMsg)
}

// --------------- InheritAllLabelsFrom ---------------.
func TestInheritAllLabelsFrom(t *testing.T) {
	containerRawEntityID := "kube-system_kube-addon-manager-minikube_kube-addon-manager"
	raw := definition.RawGroups{
		"pod": {
			"kube-system_kube-addon-manager-minikube": definition.RawMetrics{
				"kube_pod_info": Metric{
					Value: GaugeValue(1507117436),
					Labels: map[string]string{
						"pod":       "kube-addon-manager-minikube",
						"pod_ip":    "172.31.248.38",
						"namespace": "kube-system",
					},
				},
			},
		},
		"container": {
			containerRawEntityID: definition.RawMetrics{
				"kube_pod_container_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"pod":          "kube-addon-manager-minikube",
						"container_id": "docker://441e4dacbcfb2f012f2221d0f3768552ea1ccb53454da42b7b3eeaf17bbd240a",
						"namespace":    "kube-system",
					},
				},
			},
		},
	}

	fetchedValue, err := InheritAllLabelsFrom("pod", "kube_pod_info")("container", containerRawEntityID, raw)
	assert.NoError(t, err)

	expectedValue := definition.FetchedValues{"label.pod_ip": "172.31.248.38", "label.pod": "kube-addon-manager-minikube", "label.namespace": "kube-system"}
	assert.Equal(t, expectedValue, fetchedValue)
}

func TestInheritAllLabelsFrom_Namespace(t *testing.T) {
	podRawEntityID := "kube-addon-manager-minikube"
	raw := definition.RawGroups{
		"namespace": {
			"kube-system": definition.RawMetrics{
				"kube_namespace_labels": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"namespace": "kube-system",
					},
				},
			},
		},
		"pod": {
			"kube-addon-manager-minikube": definition.RawMetrics{
				"kube_pod_info": Metric{
					Value: GaugeValue(1507117436),
					Labels: map[string]string{
						"pod":       "kube-addon-manager-minikube",
						"pod_ip":    "172.31.248.38",
						"namespace": "kube-system",
					},
				},
			},
		},
	}

	fetchedValue, err := InheritAllLabelsFrom("namespace", "kube_namespace_labels")("pod", podRawEntityID, raw)
	assert.NoError(t, err)

	expectedValue := definition.FetchedValues{"label.namespace": "kube-system"}
	assert.Equal(t, expectedValue, fetchedValue)
}

func TestInheritAllLabelsFrom_FromTheSameLabelGroup(t *testing.T) {
	deploymentRawEntityID := "kube-public_newrelic-infra-monitoring"
	raw := definition.RawGroups{
		"deployment": {
			deploymentRawEntityID: definition.RawMetrics{
				"kube_deployment_labels": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"deployment": "newrelic-infra-monitoring",
						"label_app":  "newrelic-infra-monitoring",
						"namespace":  "kube-public",
					},
				},
				"kube_deployment_spec_replicas": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"deployment": "newrelic-infra-monitoring",
						"namespace":  "kube-public",
					},
				},
			},
		},
	}

	fetchedValue, err := InheritAllLabelsFrom("deployment", "kube_deployment_labels")("deployment", deploymentRawEntityID, raw)
	assert.NoError(t, err)

	expectedValue := definition.FetchedValues{"label.deployment": "newrelic-infra-monitoring", "label.namespace": "kube-public", "label.app": "newrelic-infra-monitoring"}
	assert.Equal(t, expectedValue, fetchedValue)
}

func TestInheritAllLabelsFrom_LabelNotFound(t *testing.T) {
	podRawEntityID := "kube-system_kube-addon-manager-minikube"
	raw := definition.RawGroups{
		"deployment": {
			"newrelic-infra-monitoring": definition.RawMetrics{
				"kube_deployment_labels": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"deployment": "newrelic-infra-monitoring",
						"label_app":  "newrelic-infra-monitoring",
						"namespace":  "kube-public",
					},
				},
			},
		},
		"pod": {
			"kube-system_kube-addon-manager-minikube": definition.RawMetrics{
				"kube_pod_info": Metric{
					Value: GaugeValue(1507117436),
					Labels: map[string]string{
						"pod":       "kube-addon-manager-minikube",
						"pod_ip":    "172.31.248.38",
						"namespace": "kube-system",
					},
				},
			},
		},
	}

	fetchedValue, err := InheritAllLabelsFrom("deployment", "kube_deployment_labels")("pod", podRawEntityID, raw)
	assert.Nil(t, fetchedValue)
	assert.EqualError(t, err, "cannot retrieve the entity ID of metrics to inherit labels from, got error: metric with the labels [namespace deployment] not found")
}

func TestInheritAllLabelsFrom_RelatedMetricNotFound(t *testing.T) {
	containerRawEntityID := "kube-system_kube-addon-manager-minikube_kube-addon-manager"
	raw := definition.RawGroups{
		"pod": {},
		"container": {
			containerRawEntityID: definition.RawMetrics{
				"kube_pod_container_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"pod":       "kube-addon-manager-minikube",
						"namespace": "kube-system",
					},
				},
			},
		},
	}

	expectedPodRawEntityID := "kube-system_kube-addon-manager-minikube"
	fetchedValue, err := InheritAllLabelsFrom("pod", "non_existent_metric_key")("container", containerRawEntityID, raw)
	assert.EqualError(t, err, fmt.Sprintf("related metric not found. Metric: non_existent_metric_key pod:%v", expectedPodRawEntityID))
	assert.Empty(t, fetchedValue)
}

func TestInheritAllLabelsFrom_PersistentVolume(t *testing.T) {
	// PVC is cluster-scoped, so entity id is not prefixed with namespace
	pvRawEntityID := "e2e-pv-storage"
	raw := definition.RawGroups{
		"persistentvolume": {
			pvRawEntityID: definition.RawMetrics{
				"kube_persistentvolume_labels": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"persistentvolume":              "e2e-pv-storage",
						"label_app_alayacare_com_owner": "platform",
						"label_app_alayacare_com_tier":  "critical",
						"label_environment":             "dev",
						"label_team":                    "k8-team",
					},
				},
				"kube_persistentvolume_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"persistentvolume": "e2e-pv-storage",
						"storageclass":     "e2e-pv-class",
					},
				},
			},
		},
	}

	fetchedValue, err := InheritAllLabelsFrom("persistentvolume", "kube_persistentvolume_labels")("persistentvolume", pvRawEntityID, raw)
	assert.NoError(t, err)

	expectedValue := definition.FetchedValues{
		"label.persistentvolume":        "e2e-pv-storage",
		"label.app_alayacare_com_owner": "platform",
		"label.app_alayacare_com_tier":  "critical",
		"label.environment":             "dev",
		"label.team":                    "k8-team",
	}
	assert.Equal(t, expectedValue, fetchedValue)
}

func TestInheritAllLabelsFrom_PersistentVolumeClaim(t *testing.T) {
	// PVC is namespace-scoped, so grouper creates entity ID as: namespace_pvcname
	pvcRawEntityID := "scraper_e2e-pv-claim"
	raw := definition.RawGroups{
		"persistentvolumeclaim": {
			pvcRawEntityID: definition.RawMetrics{
				"kube_persistentvolumeclaim_labels": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"namespace":                     "scraper",
						"persistentvolumeclaim":         "e2e-pv-claim",
						"label_app_alayacare_com_owner": "infrastructure",
						"label_app_alayacare_com_tier":  "high",
						"label_environment":             "staging",
						"label_team":                    "storage-team",
					},
				},
				"kube_persistentvolumeclaim_info": Metric{
					Value: GaugeValue(1),
					Labels: map[string]string{
						"namespace":             "scraper",
						"persistentvolumeclaim": "e2e-pv-claim",
						"storageclass":          "e2e-pv-class",
						"volumename":            "e2e-pv-storage",
					},
				},
			},
		},
	}

	fetchedValue, err := InheritAllLabelsFrom("persistentvolumeclaim", "kube_persistentvolumeclaim_labels")("persistentvolumeclaim", pvcRawEntityID, raw)
	assert.NoError(t, err)

	expectedValue := definition.FetchedValues{
		"label.namespace":               "scraper",
		"label.persistentvolumeclaim":   "e2e-pv-claim",
		"label.app_alayacare_com_owner": "infrastructure",
		"label.app_alayacare_com_tier":  "high",
		"label.environment":             "staging",
		"label.team":                    "storage-team",
	}
	assert.Equal(t, expectedValue, fetchedValue)
}

// TestFromMetricWithPrefixedLabels_EquivalenceWithInheritAllLabelsFrom verifies that
// FromMetricWithPrefixedLabels produces the same results as InheritAllLabelsFrom
// for same-entity label inheritance (where groupLabel == parentGroupLabel).
func getEquivalenceTestCases() []struct {
	name         string
	groupLabel   string
	entityID     string
	metricName   string
	rawGroups    definition.RawGroups
	expectedVals definition.FetchedValues
} {
	return []struct {
		name         string
		groupLabel   string
		entityID     string
		metricName   string
		rawGroups    definition.RawGroups
		expectedVals definition.FetchedValues
	}{
		{
			name:       "Deployment same-entity labels",
			groupLabel: "deployment",
			entityID:   "kube-public_newrelic-infra-monitoring",
			metricName: "kube_deployment_labels",
			rawGroups: definition.RawGroups{
				"deployment": {
					"kube-public_newrelic-infra-monitoring": definition.RawMetrics{
						"kube_deployment_labels": Metric{
							Value: GaugeValue(1),
							Labels: map[string]string{
								"deployment":    "newrelic-infra-monitoring",
								"namespace":     "kube-public",
								"label_app":     "newrelic-infra-monitoring",
								"label_version": "1.2.3",
								"label_team":    "observability",
							},
						},
					},
				},
			},
			expectedVals: definition.FetchedValues{
				"label.deployment": "newrelic-infra-monitoring",
				"label.namespace":  "kube-public",
				"label.app":        "newrelic-infra-monitoring",
				"label.version":    "1.2.3",
				"label.team":       "observability",
			},
		},
		{
			name:       "PersistentVolume same-entity labels",
			groupLabel: "persistentvolume",
			entityID:   "e2e-pv-storage",
			metricName: "kube_persistentvolume_labels",
			rawGroups: definition.RawGroups{
				"persistentvolume": {
					"e2e-pv-storage": definition.RawMetrics{
						"kube_persistentvolume_labels": Metric{
							Value: GaugeValue(1),
							Labels: map[string]string{
								"persistentvolume":              "e2e-pv-storage",
								"label_app_alayacare_com_owner": "platform",
								"label_environment":             "dev",
								"label_team":                    "k8-team",
							},
						},
					},
				},
			},
			expectedVals: definition.FetchedValues{
				"label.persistentvolume":        "e2e-pv-storage",
				"label.app_alayacare_com_owner": "platform",
				"label.environment":             "dev",
				"label.team":                    "k8-team",
			},
		},
		{
			name:       "StatefulSet same-entity labels",
			groupLabel: "statefulset",
			entityID:   "default_web",
			metricName: "kube_statefulset_labels",
			rawGroups: definition.RawGroups{
				"statefulset": {
					"default_web": definition.RawMetrics{
						"kube_statefulset_labels": Metric{
							Value: GaugeValue(1),
							Labels: map[string]string{
								"statefulset": "web",
								"namespace":   "default",
								"label_app":   "nginx",
								"label_tier":  "frontend",
							},
						},
					},
				},
			},
			expectedVals: definition.FetchedValues{
				"label.statefulset": "web",
				"label.namespace":   "default",
				"label.app":         "nginx",
				"label.tier":        "frontend",
			},
		},
	}
}

func TestFromMetricWithPrefixedLabels_EquivalenceWithInheritAllLabelsFrom(t *testing.T) {
	tests := getEquivalenceTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test InheritAllLabelsFrom (old method)
			inheritFunc := InheritAllLabelsFrom(tt.groupLabel, tt.metricName)
			inheritedValue, err := inheritFunc(tt.groupLabel, tt.entityID, tt.rawGroups)
			require.NoError(t, err, "InheritAllLabelsFrom should not error")

			// Test FromMetricWithPrefixedLabels (new method)
			prefixedFunc := FromMetricWithPrefixedLabels(tt.metricName, "label")
			prefixedValue, err := prefixedFunc(tt.groupLabel, tt.entityID, tt.rawGroups)
			require.NoError(t, err, "FromMetricWithPrefixedLabels should not error")

			// Both methods should produce identical results
			assert.Equal(t, tt.expectedVals, inheritedValue, "InheritAllLabelsFrom result")
			assert.Equal(t, tt.expectedVals, prefixedValue, "FromMetricWithPrefixedLabels result")
			assert.Equal(t, inheritedValue, prefixedValue, "Both methods should produce identical results")
		})
	}
}

func TestControlPlaneComponentTypeGenerator(t *testing.T) {
	generatedType, err := ControlPlaneComponentTypeGenerator("my-component", "", nil, "myCluster")
	assert.NoError(t, err)
	assert.Equal(t, "k8s:myCluster:controlplane:my-component", generatedType)
}

func TestFromFlattenedMetrics(t *testing.T) {
	testCases := []struct {
		name           string
		metricName     string
		metricKeyLabel string
		rawGroups      definition.RawGroups
		expectedValue  definition.FetchedValue
		expectedErr    string
	}{
		{
			name:           "Happy_Path_Unpacks_Slice",
			metricName:     "kube_resourcequota",
			metricKeyLabel: "type",
			rawGroups: definition.RawGroups{
				"resourcequota": {
					"test-entity": {
						"kube_resourcequota": []Metric{
							{Labels: Labels{"resource": "pods", "type": "hard"}, Value: GaugeValue(10)},
							{Labels: Labels{"resource": "pods", "type": "used"}, Value: GaugeValue(5)},
						},
					},
				},
			},
			expectedValue: definition.FetchedValues{
				"hard": GaugeValue(10),
				"used": GaugeValue(5),
			},
			expectedErr: "",
		},
		{
			name:           "Metric_Not_Found",
			metricName:     "non_existent_metric",
			metricKeyLabel: "type",
			rawGroups: definition.RawGroups{
				"resourcequota": {
					"test-entity": {}, // Empty RawMetrics
				},
			},
			expectedValue: nil,
			expectedErr:   `metric "non_existent_metric" not found`,
		},
		{
			name:           "Wrong_Data_Type",
			metricName:     "kube_resourcequota",
			metricKeyLabel: "type",
			rawGroups: definition.RawGroups{
				"resourcequota": {
					"test-entity": {
						"kube_resourcequota": "this is not a slice",
					},
				},
			},
			expectedValue: nil,
			expectedErr:   "", // Should return nil, nil gracefully
		},
		{
			name:           "Empty_Slice",
			metricName:     "kube_resourcequota",
			metricKeyLabel: "type",
			rawGroups: definition.RawGroups{
				"resourcequota": {
					"test-entity": {
						"kube_resourcequota": []Metric{},
					},
				},
			},
			expectedValue: nil,
			expectedErr:   "", // Should return nil, nil gracefully
		},
		{
			name:           "Metric_In_Slice_Missing_Key_Label",
			metricName:     "kube_resourcequota",
			metricKeyLabel: "type",
			rawGroups: definition.RawGroups{
				"resourcequota": {
					"test-entity": {
						"kube_resourcequota": []Metric{
							{Labels: Labels{"resource": "pods", "type": "hard"}, Value: GaugeValue(10)},
							{Labels: Labels{"resource": "pods", "other_label": "foo"}, Value: GaugeValue(5)}, // This one is missing the 'type' label
						},
					},
				},
			},
			expectedValue: definition.FetchedValues{
				"hard": GaugeValue(10), // Only the 'hard' metric should be in the result.
			},
			expectedErr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the FetchFunc using the function we are testing.
			fetchFunc := FromFlattenedMetrics(tc.metricName, tc.metricKeyLabel)

			// Execute the FetchFunc.
			fetchedValue, err := fetchFunc("resourcequota", "test-entity", tc.rawGroups)

			// Assert on the error.
			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

			// Assert on the returned value.
			assert.Equal(t, tc.expectedValue, fetchedValue)
		})
	}
}

//nolint:funlen // TestFromLabelValue is long due to comprehensive test cases.
func TestFromLabelValue(t *testing.T) {
	testCases := []struct {
		name          string
		key           string
		label         string
		rawGroups     definition.RawGroups
		expectedValue interface{}
		expectedErr   string
	}{
		{
			name:  "Success_with_single_Metric",
			key:   "kube_pod_info",
			label: "namespace",
			rawGroups: definition.RawGroups{
				"pod": {
					"test-entity": {
						"kube_pod_info": Metric{Labels: Labels{"namespace": "prod"}},
					},
				},
			},
			expectedValue: "prod",
			expectedErr:   "",
		},
		{
			name:  "Success_with_slice_of_Metrics",
			key:   "kube_pod_status",
			label: "phase",
			rawGroups: definition.RawGroups{
				"pod": {
					"test-entity": {
						"kube_pod_status": []Metric{
							{Labels: Labels{"phase": "Running", "namespace": "prod"}},
							{Labels: Labels{"phase": "Succeeded", "namespace": "prod"}},
						},
					},
				},
			},
			expectedValue: "Running",
			expectedErr:   "",
		},
		{
			name:  "Error_when_metric_not_found",
			key:   "non_existent_metric",
			label: "namespace",
			rawGroups: definition.RawGroups{
				"pod": {"test-entity": {}},
			},
			expectedValue: nil,
			expectedErr:   `metric "non_existent_metric" not found`,
		},
		{
			name:  "Error_when_label_not_found_in_single_metric",
			key:   "kube_pod_info",
			label: "missing_label",
			rawGroups: definition.RawGroups{
				"pod": {
					"test-entity": {
						"kube_pod_info": Metric{Labels: Labels{"namespace": "prod"}},
					},
				},
			},
			expectedValue: nil,
			expectedErr:   `label "missing_label" not found on metric "kube_pod_info": label not found on metric`,
		},
		{
			name:  "Error_when_label_not_found_in_slice",
			key:   "kube_pod_status",
			label: "missing_label",
			rawGroups: definition.RawGroups{
				"pod": {
					"test-entity": {
						"kube_pod_status": []Metric{
							{Labels: Labels{"phase": "Running"}},
						},
					},
				},
			},
			expectedValue: nil,
			expectedErr:   `label "missing_label" not found in the first metric for key "kube_pod_status": label not found in the first metric for key`,
		},
		{
			name:  "Error_when_slice_is_empty",
			key:   "kube_pod_status",
			label: "phase",
			rawGroups: definition.RawGroups{
				"pod": {
					"test-entity": {
						"kube_pod_status": []Metric{},
					},
				},
			},
			expectedValue: nil,
			expectedErr:   `metric slice for key "kube_pod_status" was empty: metric slice for key was empty`,
		},
		{
			name:  "Error_on_incompatible_type",
			key:   "kube_pod_info",
			label: "namespace",
			rawGroups: definition.RawGroups{
				"pod": {
					"test-entity": {
						"kube_pod_info": "this is not a metric",
					},
				},
			},
			expectedValue: nil,
			expectedErr:   `incompatible metric type for "kube_pod_info". Expected: Metric or []Metric. Got: string: incompatible metric type for key`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the FetchFunc using the function we are testing.
			fetchFunc := FromLabelValue(tc.key, tc.label)

			// Execute the FetchFunc.
			fetchedValue, err := fetchFunc("pod", "test-entity", tc.rawGroups)

			// Assert on the error.
			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

			// Assert on the returned value.
			assert.Equal(t, tc.expectedValue, fetchedValue)
		})
	}
}

func TestFromMetricWithPrefixedLabels(t *testing.T) {
	testCases := []struct {
		name          string
		metricName    string
		prefix        string
		rawGroups     definition.RawGroups
		expectedValue definition.FetchedValue
		expectedErr   string
	}{
		{
			name:       "Happy_Path_Extracts_And_Formats_Labels",
			metricName: "kube_pod_labels",
			prefix:     "label",
			rawGroups: definition.RawGroups{
				"pod": {
					"test-entity": {
						"kube_pod_labels": Metric{
							Labels: Labels{
								"label_app":  "my-app",
								"label_team": "sre",
								"namespace":  "prod", // This label is now included too.
							},
						},
					},
				},
			},
			expectedValue: definition.FetchedValues{
				"label.app":       "my-app",
				"label.team":      "sre",
				"label.namespace": "prod", // Now includes all labels.
			},
			expectedErr: "",
		},
		{
			name:       "No_Matching_Prefixed_Labels",
			metricName: "kube_pod_labels",
			prefix:     "label",
			rawGroups: definition.RawGroups{
				"pod": {
					"test-entity": {
						"kube_pod_labels": Metric{
							Labels: Labels{"namespace": "prod"}, // No labels with "label_" prefix, but still included.
						},
					},
				},
			},
			expectedValue: definition.FetchedValues{"label.namespace": "prod"}, // Now includes all labels.
			expectedErr:   "",
		},
		{
			name:       "Metric_Not_Found_Is_Handled_Gracefully",
			metricName: "non_existent_metric",
			prefix:     "label",
			rawGroups: definition.RawGroups{
				"pod": {"test-entity": {}},
			},
			expectedValue: nil, // Expect nil for both value and error.
			expectedErr:   "",
		},
		{
			name:       "Error_On_Incompatible_Data_Type",
			metricName: "kube_pod_labels",
			prefix:     "label",
			rawGroups: definition.RawGroups{
				"pod": {
					"test-entity": {
						"kube_pod_labels": "this is not a Metric object",
					},
				},
			},
			expectedValue: nil,
			expectedErr:   `expected metric type for "kube_pod_labels" to be Metric, but got string: expected metric type for key to be Metric`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the FetchFunc using the function we are testing.
			fetchFunc := FromMetricWithPrefixedLabels(tc.metricName, tc.prefix)

			// Execute the FetchFunc.
			fetchedValue, err := fetchFunc("pod", "test-entity", tc.rawGroups)

			// Assert on the error.
			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

			// Assert on the returned value.
			assert.Equal(t, tc.expectedValue, fetchedValue)
		})
	}
}
func TestFromValue_EndpointAddressAvailableAndNotReady(t *testing.T) {
	// Test data for kube_endpoint_address_available and kube_endpoint_address_not_ready metrics
	// These metrics were introduced in KSM < 2.14 and provide aggregate counts per endpoint
	// Unlike kube_endpoint_address, these only have namespace and endpoint labels (no ip, port details)
	rawGroups := definition.RawGroups{
		"endpoint": {
			"kube-system_kube-dns": {
				"kube_endpoint_address_available": []Metric{
					{
						Labels: Labels{
							"namespace": "kube-system",
							"endpoint":  "kube-dns",
						},
						Value: GaugeValue(2),
					},
				},
				"kube_endpoint_address_not_ready": []Metric{
					{
						Labels: Labels{
							"namespace": "kube-system",
							"endpoint":  "kube-dns",
						},
						Value: GaugeValue(1),
					},
				},
			},
			"nr-test11_test11-resources-hpa": {
				"kube_endpoint_address_available": []Metric{
					{
						Labels: Labels{
							"namespace": "nr-test11",
							"endpoint":  "test11-resources-hpa",
						},
						Value: GaugeValue(1),
					},
				},
				"kube_endpoint_address_not_ready": []Metric{
					{
						Labels: Labels{
							"namespace": "nr-test11",
							"endpoint":  "test11-resources-hpa",
						},
						Value: GaugeValue(0),
					},
				},
			},
			"nr-test11_test11-resources-statefulset": {
				"kube_endpoint_address_available": []Metric{
					{
						Labels: Labels{
							"namespace": "nr-test11",
							"endpoint":  "test11-resources-statefulset",
						},
						Value: GaugeValue(2),
					},
				},
				"kube_endpoint_address_not_ready": []Metric{
					{
						Labels: Labels{
							"namespace": "nr-test11",
							"endpoint":  "test11-resources-statefulset",
						},
						Value: GaugeValue(0),
					},
				},
			},
			"default_kubernetes": {
				"kube_endpoint_address_available": []Metric{
					{
						Labels: Labels{
							"namespace": "default",
							"endpoint":  "kubernetes",
						},
						Value: GaugeValue(1),
					},
				},
				"kube_endpoint_address_not_ready": []Metric{
					{
						Labels: Labels{
							"namespace": "default",
							"endpoint":  "kubernetes",
						},
						Value: GaugeValue(0),
					},
				},
			},
			"kube-system_k8s.io-minikube-hostpath": {
				"kube_endpoint_address_available": []Metric{
					{
						Labels: Labels{
							"namespace": "kube-system",
							"endpoint":  "k8s.io-minikube-hostpath",
						},
						Value: GaugeValue(0),
					},
				},
				"kube_endpoint_address_not_ready": []Metric{
					{
						Labels: Labels{
							"namespace": "kube-system",
							"endpoint":  "k8s.io-minikube-hostpath",
						},
						Value: GaugeValue(0),
					},
				},
			},
		},
	}

	testCases := []struct {
		name          string
		fetchFunc     definition.FetchFunc
		entityID      string
		expectedValue GaugeValue
		description   string
	}{
		{
			name:          "FromValue kube_endpoint_address_available - multiple addresses",
			fetchFunc:     FromValue("kube_endpoint_address_available"),
			entityID:      "kube-system_kube-dns",
			expectedValue: GaugeValue(2),
			description:   "Should fetch addressAvailable with count of 2 for kube-dns endpoint",
		},
		{
			name:          "FromValue kube_endpoint_address_not_ready - zero value",
			fetchFunc:     FromValue("kube_endpoint_address_not_ready"),
			entityID:      "kube-system_kube-dns",
			expectedValue: GaugeValue(1),
			description:   "Should fetch addressNotReady with count of 1 for kube-dns endpoint",
		},
		{
			name:          "FromValue kube_endpoint_address_available - single address",
			fetchFunc:     FromValue("kube_endpoint_address_available"),
			entityID:      "default_kubernetes",
			expectedValue: GaugeValue(1),
			description:   "Should fetch addressAvailable with count of 1 for kubernetes endpoint",
		},
		{
			name:          "FromValue kube_endpoint_address_not_ready - kubernetes",
			fetchFunc:     FromValue("kube_endpoint_address_not_ready"),
			entityID:      "default_kubernetes",
			expectedValue: GaugeValue(0),
			description:   "Should fetch addressNotReady with count of 0 for kubernetes endpoint",
		},
		{
			name:          "FromValue kube_endpoint_address_available - statefulset with 2 addresses",
			fetchFunc:     FromValue("kube_endpoint_address_available"),
			entityID:      "nr-test11_test11-resources-statefulset",
			expectedValue: GaugeValue(2),
			description:   "Should fetch addressAvailable with count of 2 for statefulset endpoint",
		},
		{
			name:          "FromValue kube_endpoint_address_available - zero available addresses",
			fetchFunc:     FromValue("kube_endpoint_address_available"),
			entityID:      "kube-system_k8s.io-minikube-hostpath",
			expectedValue: GaugeValue(0),
			description:   "Should fetch addressAvailable with count of 0 when no addresses available",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.fetchFunc("endpoint", tc.entityID, rawGroups)
			require.NoError(t, err, tc.description)

			// FromValue with []Metric returns FetchedValues (map)
			fetchedValues, ok := result.(definition.FetchedValues)
			require.True(t, ok, "Expected result to be of type FetchedValues, got %T", result)

			// Should have exactly one entry in the map
			require.Equal(t, 1, len(fetchedValues), "Expected exactly one metric in FetchedValues")

			// Get the single value from the map and verify it matches expected
			for _, val := range fetchedValues {
				gaugeValue, ok := val.(GaugeValue)
				require.True(t, ok, "Expected value to be of type GaugeValue")
				assert.Equal(t, tc.expectedValue, gaugeValue, tc.description)
			}
		})
	}
}

func TestCountFromValueWithLabelsFilter_kubepointAddress(t *testing.T) {
	// Test that FromValueWithLabelsFilter on kube_endpoint_address produces the same
	// addressAvailable and addressNotReady counts as the dedicated metrics
	// This demonstrates KSM >= 2.14 compatibility
	rawGroups := definition.RawGroups{
		"endpoint": {
			"kube-system_kube-dns": {
				"kube_endpoint_address": []Metric{
					{
						Labels: Labels{
							"namespace":     "kube-system",
							"endpoint":      "kube-dns",
							"port_protocol": "TCP",
							"port_number":   "53",
							"port_name":     "dns-tcp",
							"ip":            "10.244.0.2",
							"ready":         "true",
						},
						Value: GaugeValue(1),
					},
					{
						Labels: Labels{
							"namespace":     "kube-system",
							"endpoint":      "kube-dns",
							"port_protocol": "UDP",
							"port_number":   "53",
							"port_name":     "dns",
							"ip":            "10.244.0.2",
							"ready":         "true",
						},
						Value: GaugeValue(1),
					},
					{
						Labels: Labels{
							"namespace":     "kube-system",
							"endpoint":      "kube-dns",
							"port_protocol": "TCP",
							"port_number":   "9153",
							"port_name":     "metrics",
							"ip":            "10.244.0.2",
							"ready":         "true",
						},
						Value: GaugeValue(1),
					},
					{
						Labels: Labels{
							"namespace":     "kube-system",
							"endpoint":      "kube-dns",
							"port_protocol": "TCP",
							"port_number":   "53",
							"port_name":     "dns-tcp",
							"ip":            "10.244.0.3",
							"ready":         "false",
						},
						Value: GaugeValue(1),
					},
				},
			},
			"nr-test11_test11-resources-statefulset": {
				"kube_endpoint_address": []Metric{
					{
						Labels: Labels{
							"namespace":     "nr-test11",
							"endpoint":      "test11-resources-statefulset",
							"port_protocol": "TCP",
							"port_number":   "8089",
							"port_name":     "",
							"ip":            "10.244.0.14",
							"ready":         "true",
						},
						Value: GaugeValue(1),
					},
					{
						Labels: Labels{
							"namespace":     "nr-test11",
							"endpoint":      "test11-resources-statefulset",
							"port_protocol": "TCP",
							"port_number":   "8089",
							"port_name":     "",
							"ip":            "10.244.0.15",
							"ready":         "true",
						},
						Value: GaugeValue(1),
					},
				},
			},
			"default_kubernetes": {
				"kube_endpoint_address": []Metric{
					{
						Labels: Labels{
							"namespace":     "default",
							"endpoint":      "kubernetes",
							"port_protocol": "TCP",
							"port_number":   "8443",
							"port_name":     "https",
							"ip":            "192.168.49.2",
							"ready":         "true",
						},
						Value: GaugeValue(1),
					},
				},
			},
			"nr-test11_test11-resources-hpa": {
				"kube_endpoint_address": []Metric{
					{
						Labels: Labels{
							"namespace":     "nr-test11",
							"endpoint":      "test11-resources-hpa",
							"port_protocol": "TCP",
							"port_number":   "80",
							"port_name":     "",
							"ip":            "10.244.0.5",
							"ready":         "true",
						},
						Value: GaugeValue(1),
					},
				},
			},
			"kube-system_k8s.io-minikube-hostpath": {
				"kube_endpoint_address": []Metric{
					{
						Labels: Labels{
							"namespace":     "kube-system",
							"endpoint":      "k8s.io-minikube-hostpath",
							"port_protocol": "TCP",
							"port_number":   "80",
							"port_name":     "",
							"ip":            "10.244.0.20",
							"ready":         "false",
						},
						Value: GaugeValue(1),
					},
				},
			},
		},
	}

	testCases := []struct {
		name          string
		fetchFunc     definition.FetchFunc
		entityID      string
		expectedValue GaugeValue
		description   string
	}{
		{
			name: "CountFromValueWithLabelsFilter addressAvailable - kube-dns with 3 ready addresses",
			fetchFunc: CountFromValueWithLabelsFilter(
				"kube_endpoint_address",
				"addressAvailable",
				IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "true"}),
			),
			entityID:      "kube-system_kube-dns",
			expectedValue: GaugeValue(3),
			description:   "Should count 3 addresses with ready=true for kube-dns",
		},
		{
			name: "CountFromValueWithLabelsFilter addressNotReady - kube-dns with 1 not ready address",
			fetchFunc: CountFromValueWithLabelsFilter(
				"kube_endpoint_address",
				"addressNotReady",
				IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "false"}),
			),
			entityID:      "kube-system_kube-dns",
			expectedValue: GaugeValue(1),
			description:   "Should count 1 address with ready=false for kube-dns",
		},
		{
			name: "CountFromValueWithLabelsFilter addressAvailable - statefulset with 2 ready addresses",
			fetchFunc: CountFromValueWithLabelsFilter(
				"kube_endpoint_address",
				"addressAvailable",
				IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "true"}),
			),
			entityID:      "nr-test11_test11-resources-statefulset",
			expectedValue: GaugeValue(2),
			description:   "Should count 2 addresses with ready=true for statefulset",
		},
		{
			name: "CountFromValueWithLabelsFilter addressAvailable - kubernetes with 1 ready address",
			fetchFunc: CountFromValueWithLabelsFilter(
				"kube_endpoint_address",
				"addressAvailable",
				IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "true"}),
			),
			entityID:      "default_kubernetes",
			expectedValue: GaugeValue(1),
			description:   "Should count 1 address with ready=true for kubernetes",
		},
		{
			name: "CountFromValueWithLabelsFilter addressNotReady - kubernetes with 0 not ready addresses",
			fetchFunc: CountFromValueWithLabelsFilter(
				"kube_endpoint_address",
				"addressNotReady",
				IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "false"}),
			),
			entityID:      "default_kubernetes",
			expectedValue: GaugeValue(0),
			description:   "Should return 0 when no addresses have ready=false",
		},
		{
			name: "CountFromValueWithLabelsFilter addressAvailable - endpoint with 0 ready addresses",
			fetchFunc: CountFromValueWithLabelsFilter(
				"kube_endpoint_address",
				"addressAvailable",
				IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "true"}),
			),
			entityID:      "kube-system_k8s.io-minikube-hostpath",
			expectedValue: GaugeValue(0),
			description:   "Should return 0 when no addresses have ready=true",
		},
		{
			name: "CountFromValueWithLabelsFilter addressNotReady - endpoint with 1 not ready address",
			fetchFunc: CountFromValueWithLabelsFilter(
				"kube_endpoint_address",
				"addressNotReady",
				IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "false"}),
			),
			entityID:      "kube-system_k8s.io-minikube-hostpath",
			expectedValue: GaugeValue(1),
			description:   "Should count 1 address with ready=false",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.fetchFunc("endpoint", tc.entityID, rawGroups)
			require.NoError(t, err, tc.description)

			fetchedValues, ok := result.(definition.FetchedValues)
			require.True(t, ok, "Expected result to be of type FetchedValues, got %T", result)

			// When the filter matches nothing, we get an empty map
			if tc.expectedValue == 0 {
				// Either empty map or a map with a single entry of value 0
				if len(fetchedValues) == 0 {
					// This is valid - no matching metrics
					return
				}
			}

			// Should have exactly one entry in the map
			require.Equal(t, 1, len(fetchedValues), "Expected exactly one metric in FetchedValues")

			// Get the single value from the map and verify it matches expected
			for _, val := range fetchedValues {
				gaugeValue, ok := val.(GaugeValue)
				require.True(t, ok, "Expected value to be of type GaugeValue")
				assert.Equal(t, tc.expectedValue, gaugeValue, tc.description)
			}
		})
	}
}

// TestCountFromValueWithLabelsFilter_MetricDoesNotExist tests that CountFromValueWithLabelsFilter
// returns 0 when the underlying metric doesn't exist at all.
// Real-world scenario: k8s.io-minikube-hostpath service exists but has no backing pods,
// so no kube_endpoint_address metric exists in KSM v2.16+ data.
func TestCountFromValueWithLabelsFilter_MetricDoesNotExist(t *testing.T) {
	t.Parallel()

	// Test data with endpoint entity but NO kube_endpoint_address metric
	rawGroups := definition.RawGroups{
		"endpoint": {
			"kube-system_k8s.io-minikube-hostpath": {
				"kube_endpoint_info": Metric{
					Labels: Labels{
						"namespace": "kube-system",
						"endpoint":  "k8s.io-minikube-hostpath",
					},
					Value: GaugeValue(1),
				},
				// NOTE: No kube_endpoint_address metric at all - endpoint has 0 addresses
			},
		},
	}

	// Create the count function - expects kube_endpoint_address with ready="true"
	countFunc := CountFromValueWithLabelsFilter(
		"kube_endpoint_address",
		"addressAvailable",
		IncludeOnlyWhenLabelMatchFilter(map[string]string{
			"ready": "true",
		}),
	)

	// Execute the function
	result, err := countFunc("endpoint", "kube-system_k8s.io-minikube-hostpath", rawGroups)

	// Should NOT return an error - should return 0
	require.NoError(t, err, "CountFromValueWithLabelsFilter should return 0 (not error) when metric doesn't exist")

	// Verify the result is FetchedValues with 0
	fetchedValues, ok := result.(definition.FetchedValues)
	require.True(t, ok, "Result should be FetchedValues")
	require.Len(t, fetchedValues, 1, "Should have exactly one entry")

	val, exists := fetchedValues["addressAvailable"]
	require.True(t, exists, "Should have addressAvailable key")

	gaugeValue, ok := val.(GaugeValue)
	require.True(t, ok, "Value should be GaugeValue")
	assert.Equal(t, GaugeValue(0), gaugeValue, "Should return 0 when metric doesn't exist")
}

// TestCountFromValueWithLabelsFilter_NoMatchingLabels tests that CountFromValueWithLabelsFilter
// returns 0 when the metric exists but no entries match the label filter.
// Real-world scenario: Endpoint has addresses, but all are ready=true, so filtering for ready=false returns 0.
func TestCountFromValueWithLabelsFilter_NoMatchingLabels(t *testing.T) {
	t.Parallel()

	// Test data with endpoint that has addresses, but ALL are ready=true
	rawGroups := definition.RawGroups{
		"endpoint": {
			"default_kubernetes": {
				"kube_endpoint_address": []Metric{
					{
						Labels: Labels{
							"namespace": "default",
							"endpoint":  "kubernetes",
							"ip":        "192.168.1.1",
							"ready":     "true", // All addresses are ready
						},
						Value: GaugeValue(1),
					},
					{
						Labels: Labels{
							"namespace": "default",
							"endpoint":  "kubernetes",
							"ip":        "192.168.1.2",
							"ready":     "true", // All addresses are ready
						},
						Value: GaugeValue(1),
					},
				},
			},
		},
	}

	// Create the count function - looking for ready="false" addresses
	countFunc := CountFromValueWithLabelsFilter(
		"kube_endpoint_address",
		"addressNotReady",
		IncludeOnlyWhenLabelMatchFilter(map[string]string{
			"ready": "false", // Looking for NOT ready addresses
		}),
	)

	// Execute the function
	result, err := countFunc("endpoint", "default_kubernetes", rawGroups)

	// Should NOT return an error - should return 0
	require.NoError(t, err, "CountFromValueWithLabelsFilter should return 0 (not error) when no labels match")

	// Verify the result is FetchedValues with 0
	fetchedValues, ok := result.(definition.FetchedValues)
	require.True(t, ok, "Result should be FetchedValues")
	require.Len(t, fetchedValues, 1, "Should have exactly one entry")

	val, exists := fetchedValues["addressNotReady"]
	require.True(t, exists, "Should have addressNotReady key")

	gaugeValue, ok := val.(GaugeValue)
	require.True(t, ok, "Value should be GaugeValue")
	assert.Equal(t, GaugeValue(0), gaugeValue, "Should return 0 when no addresses match ready=false")
}

// TestCountFromValueWithLabelsFilter_BackwardCompatibility_Scenario1 tests that both KSM v2.13 and v2.16
// data formats produce the same output (0) for Scenario 1: endpoint with no addresses.
func TestCountFromValueWithLabelsFilter_BackwardCompatibility_Scenario1(t *testing.T) {
	t.Parallel()

	entityID := "kube-system_k8s.io-minikube-hostpath"

	// Scenario 1a: KSM v2.13 - explicit 0 value
	ksmV213Data := definition.RawGroups{
		"endpoint": {
			entityID: {
				"kube_endpoint_address_available": Metric{
					Labels: Labels{"namespace": "kube-system", "endpoint": "k8s.io-minikube-hostpath"},
					Value:  GaugeValue(0),
				},
			},
		},
	}

	// Scenario 1b: KSM v2.16 - metric doesn't exist at all
	ksmV216Data := definition.RawGroups{
		"endpoint": {
			entityID: {
				"kube_endpoint_info": Metric{
					Labels: Labels{"namespace": "kube-system", "endpoint": "k8s.io-minikube-hostpath"},
					Value:  GaugeValue(1),
				},
			},
		},
	}

	t.Run("Scenario_1a_v2.13_explicit_zero", func(t *testing.T) {
		t.Parallel()
		oldSpec := FromValue("kube_endpoint_address_available")
		result, err := oldSpec("endpoint", entityID, ksmV213Data)
		require.NoError(t, err)

		gaugeValue, ok := result.(GaugeValue)
		require.True(t, ok, "result should be GaugeValue")
		assert.Equal(t, GaugeValue(0), gaugeValue, "v2.13: OLD spec returns explicit 0")
	})

	t.Run("Scenario_1b_v2.16_metric_missing", func(t *testing.T) {
		t.Parallel()
		newSpec := CountFromValueWithLabelsFilter("kube_endpoint_address", "addressAvailable",
			IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "true"}))
		result, err := newSpec("endpoint", entityID, ksmV216Data)
		require.NoError(t, err)

		fetchedValues, ok := result.(definition.FetchedValues)
		require.True(t, ok, "result should be FetchedValues")
		assert.Equal(t, GaugeValue(0), fetchedValues["addressAvailable"], "v2.16: NEW spec returns 0 when metric missing")
	})

	t.Run("Both_produce_same_output", func(t *testing.T) {
		t.Parallel()
		// v2.13
		oldSpec := FromValue("kube_endpoint_address_available")
		resultV213, _ := oldSpec("endpoint", entityID, ksmV213Data)
		v213Value, ok := resultV213.(GaugeValue)
		require.True(t, ok, "v2.13 result should be GaugeValue")

		// v2.16
		newSpec := CountFromValueWithLabelsFilter("kube_endpoint_address", "addressAvailable",
			IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "true"}))
		resultV216, _ := newSpec("endpoint", entityID, ksmV216Data)
		v216Value := resultV216.(definition.FetchedValues)["addressAvailable"]

		assert.Equal(t, v213Value, v216Value, "Both KSM versions produce same output: 0")
	})
}

// TestCountFromValueWithLabelsFilter_BackwardCompatibility_Scenario2 tests that both KSM v2.13 and v2.16
// data formats produce the same output (0) for Scenario 2: all addresses are ready (none are not-ready).
func TestCountFromValueWithLabelsFilter_BackwardCompatibility_Scenario2(t *testing.T) {
	t.Parallel()

	entityID := "default_kubernetes"

	// Scenario 2a: KSM v2.13 - explicit 0 for not-ready
	ksmV213Data := definition.RawGroups{
		"endpoint": {
			entityID: {
				"kube_endpoint_address_not_ready": Metric{
					Labels: Labels{"namespace": "default", "endpoint": "kubernetes"},
					Value:  GaugeValue(0),
				},
			},
		},
	}

	// Scenario 2b: KSM v2.16 - all addresses have ready="true"
	ksmV216Data := definition.RawGroups{
		"endpoint": {
			entityID: {
				"kube_endpoint_address": []Metric{
					{Labels: Labels{"namespace": "default", "endpoint": "kubernetes", "ip": "192.168.1.1", "ready": "true"}, Value: GaugeValue(1)},
					{Labels: Labels{"namespace": "default", "endpoint": "kubernetes", "ip": "192.168.1.2", "ready": "true"}, Value: GaugeValue(1)},
				},
			},
		},
	}

	t.Run("Scenario_2a_v2.13_explicit_zero", func(t *testing.T) {
		t.Parallel()
		oldSpec := FromValue("kube_endpoint_address_not_ready")
		result, err := oldSpec("endpoint", entityID, ksmV213Data)
		require.NoError(t, err)

		gaugeValue := result.(GaugeValue)
		assert.Equal(t, GaugeValue(0), gaugeValue, "v2.13: OLD spec returns explicit 0 for not-ready")
	})

	t.Run("Scenario_2b_v2.16_no_matching_labels", func(t *testing.T) {
		t.Parallel()
		newSpec := CountFromValueWithLabelsFilter("kube_endpoint_address", "addressNotReady",
			IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "false"}))
		result, err := newSpec("endpoint", entityID, ksmV216Data)
		require.NoError(t, err)

		fetchedValues := result.(definition.FetchedValues)
		assert.Equal(t, GaugeValue(0), fetchedValues["addressNotReady"], "v2.16: NEW spec returns 0 when no labels match ready=false")
	})

	t.Run("Both_produce_same_output", func(t *testing.T) {
		t.Parallel()
		// v2.13
		oldSpec := FromValue("kube_endpoint_address_not_ready")
		resultV213, _ := oldSpec("endpoint", entityID, ksmV213Data)
		v213Value := resultV213.(GaugeValue)

		// v2.16
		newSpec := CountFromValueWithLabelsFilter("kube_endpoint_address", "addressNotReady",
			IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "false"}))
		resultV216, _ := newSpec("endpoint", entityID, ksmV216Data)
		v216Value := resultV216.(definition.FetchedValues)["addressNotReady"]

		assert.Equal(t, v213Value, v216Value, "Both KSM versions produce same output: 0")
	})
}

// TestFromValueWithLabelsFilter_SingleMetric tests that FromValueWithLabelsFilter correctly handles
// a single Metric value (not an array). This covers lines 687-688 in definition.go.
func TestFromValueWithLabelsFilter_SingleMetric(t *testing.T) {
	t.Parallel()

	// Test data with a single Metric (not []Metric)
	rawGroups := definition.RawGroups{
		"pod": {
			"default_my-pod": {
				"kube_pod_status_phase": Metric{
					Labels: Labels{
						"namespace": "default",
						"pod":       "my-pod",
						"phase":     "Running",
					},
					Value: GaugeValue(1),
				},
			},
		},
	}

	t.Run("Single_Metric_without_labels_filter", func(t *testing.T) {
		// FromValueWithLabelsFilter with no label filters on a single Metric
		fetchFunc := FromValueWithLabelsFilter("kube_pod_status_phase", "")
		result, err := fetchFunc("pod", "default_my-pod", rawGroups)

		require.NoError(t, err)

		// When the value is a single Metric (not []Metric), it returns the Value directly
		gaugeValue, ok := result.(GaugeValue)
		require.True(t, ok, "Expected result to be GaugeValue, got %T", result)
		assert.Equal(t, GaugeValue(1), gaugeValue)
	})

	t.Run("Single_Metric_with_labels_filter", func(t *testing.T) {
		// FromValueWithLabelsFilter with label filter on a single Metric
		// Note: Label filters only apply to []Metric, not single Metric
		fetchFunc := FromValueWithLabelsFilter("kube_pod_status_phase", "",
			IncludeOnlyWhenLabelMatchFilter(map[string]string{"phase": "Running"}))
		result, err := fetchFunc("pod", "default_my-pod", rawGroups)

		require.NoError(t, err)

		// Single Metric returns the value directly, ignoring label filters
		gaugeValue, ok := result.(GaugeValue)
		require.True(t, ok, "Expected result to be GaugeValue, got %T", result)
		assert.Equal(t, GaugeValue(1), gaugeValue)
	})
}

// TestFromValueWithLabelsFilter_IncompatibleType tests that FromValueWithLabelsFilter returns
// an error when the value is neither Metric nor []Metric. This covers lines 692-693 in definition.go.
func TestFromValueWithLabelsFilter_IncompatibleType(t *testing.T) {
	t.Parallel()

	// Test data with an incompatible type (string instead of Metric or []Metric)
	rawGroups := definition.RawGroups{
		"pod": {
			"default_my-pod": {
				"some_string_metric": "not a metric",
			},
		},
	}

	fetchFunc := FromValueWithLabelsFilter("some_string_metric", "")
	result, err := fetchFunc("pod", "default_my-pod", rawGroups)

	// Should return an error about incompatible type
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incompatible metric type for some_string_metric")
	assert.Contains(t, err.Error(), "Expected: Metric or []Metric")
	assert.Nil(t, result)
}

// TestCountFromValueWithLabelsFilter_SingleMetric tests that CountFromValueWithLabelsFilter correctly handles
// a single Metric value (not an array). This covers lines 786-787 in definition.go.
func TestCountFromValueWithLabelsFilter_SingleMetric(t *testing.T) {
	t.Parallel()

	// Test data with a single Metric (not []Metric)
	rawGroups := definition.RawGroups{
		"pod": {
			"default_my-pod": {
				"kube_pod_status_phase": Metric{
					Labels: Labels{
						"namespace": "default",
						"pod":       "my-pod",
						"phase":     "Running",
					},
					Value: GaugeValue(1),
				},
			},
		},
	}

	t.Run("Single_Metric_without_labels_filter", func(t *testing.T) {
		// CountFromValueWithLabelsFilter with no label filters on a single Metric
		fetchFunc := CountFromValueWithLabelsFilter("kube_pod_status_phase", "")
		result, err := fetchFunc("pod", "default_my-pod", rawGroups)

		require.NoError(t, err)

		// When the value is a single Metric (not []Metric), it returns the Value directly
		gaugeValue, ok := result.(GaugeValue)
		require.True(t, ok, "Expected result to be GaugeValue, got %T", result)
		assert.Equal(t, GaugeValue(1), gaugeValue)
	})

	t.Run("Single_Metric_with_labels_filter", func(t *testing.T) {
		// CountFromValueWithLabelsFilter with label filter on a single Metric
		// Note: Label filters only apply to []Metric, not single Metric
		fetchFunc := CountFromValueWithLabelsFilter("kube_pod_status_phase", "",
			IncludeOnlyWhenLabelMatchFilter(map[string]string{"phase": "Running"}))
		result, err := fetchFunc("pod", "default_my-pod", rawGroups)

		require.NoError(t, err)

		// Single Metric returns the value directly, ignoring label filters
		gaugeValue, ok := result.(GaugeValue)
		require.True(t, ok, "Expected result to be GaugeValue, got %T", result)
		assert.Equal(t, GaugeValue(1), gaugeValue)
	})
}

// TestCountFromValueWithLabelsFilter_IncompatibleType tests that CountFromValueWithLabelsFilter returns
// an error when the value is neither Metric nor []Metric. This covers lines 791-795 in definition.go.
func TestCountFromValueWithLabelsFilter_IncompatibleType(t *testing.T) {
	t.Parallel()

	// Test data with an incompatible type (string instead of Metric or []Metric)
	rawGroups := definition.RawGroups{
		"pod": {
			"default_my-pod": {
				"some_string_metric": "not a metric",
			},
		},
	}

	fetchFunc := CountFromValueWithLabelsFilter("some_string_metric", "")
	result, err := fetchFunc("pod", "default_my-pod", rawGroups)

	// Should return an error about incompatible type
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incompatible metric type for some_string_metric")
	assert.Contains(t, err.Error(), "Expected: Metric or []Metric")
	assert.Nil(t, result)
}

// TestCountFromValueWithLabelsFilter_PropagatesNonMetricErrors tests that CountFromValueWithLabelsFilter
// correctly propagates errors that are NOT "metric not found" errors. This covers line 782 in definition.go.
func TestCountFromValueWithLabelsFilter_PropagatesNonMetricErrors(t *testing.T) {
	t.Parallel()

	t.Run("Entity_not_found_error", func(t *testing.T) {
		// Test data without the entity we're looking for
		rawGroups := definition.RawGroups{
			"endpoint": {
				"kube-system_kube-dns": {
					"kube_endpoint_address": []Metric{
						{
							Labels: Labels{"namespace": "kube-system", "endpoint": "kube-dns"},
							Value:  GaugeValue(1),
						},
					},
				},
			},
		}

		fetchFunc := CountFromValueWithLabelsFilter("kube_endpoint_address", "addressAvailable",
			IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "true"}))

		// Try to fetch from a non-existent entity
		result, err := fetchFunc("endpoint", "default_nonexistent", rawGroups)

		// Should return an error (not return 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "default_nonexistent")
		assert.Nil(t, result)
	})

	t.Run("Group_not_found_error", func(t *testing.T) {
		// Test data without the group we're looking for
		rawGroups := definition.RawGroups{
			"endpoint": {
				"kube-system_kube-dns": {
					"kube_endpoint_address": []Metric{
						{
							Labels: Labels{"namespace": "kube-system", "endpoint": "kube-dns"},
							Value:  GaugeValue(1),
						},
					},
				},
			},
		}

		fetchFunc := CountFromValueWithLabelsFilter("kube_endpoint_address", "addressAvailable",
			IncludeOnlyWhenLabelMatchFilter(map[string]string{"ready": "true"}))

		// Try to fetch from a non-existent group
		result, err := fetchFunc("pod", "default_my-pod", rawGroups)

		// Should return an error (not return 0)
		require.Error(t, err)
		assert.Nil(t, result)
	})
}
