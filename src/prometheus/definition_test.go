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

// --------------- GroupMetricsBySpec ---------------
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

// --------------- FromValue ---------------
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

// --------------- FromLabelValue ---------------
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
	assert.EqualError(t, err, "incompatible metric type. Expected: Metric. Got: string")
}

func TestFromRawLabelValue_LabelNotFoundInRawMetric(t *testing.T) {
	fetchedValue, err := FromLabelValue("kube_pod_start_time", "foo")("pod", "fluentd-elasticsearch-jnqb7", rawGroups)
	assert.Nil(t, fetchedValue)
	assert.EqualError(t, err, "label not found in prometheus metric")
}

// --------------- FromLabelValueEntityTypeGenerator -------------
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
	assert.EqualError(t, err, "cannot fetch label \"namespace\" for metric \"kube_replicaset_created\": label not found in prometheus metric")
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

// --------------- FromLabelValueEntityIDGenerator ---------------
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

// --------------- FromLabelsValueEntityIDGeneratorForPendingPods ---------------
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

// --------------- InheritAllSelectorsFrom ---------------
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

// --------------- InheritAllLabelsFrom ---------------
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

func TestControlPlaneComponentTypeGenerator(t *testing.T) {
	generatedType, err := ControlPlaneComponentTypeGenerator("my-component", "", nil, "myCluster")
	assert.NoError(t, err)
	assert.Equal(t, "k8s:myCluster:controlplane:my-component", generatedType)
}
