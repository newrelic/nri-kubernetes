package metric

import (
	"fmt"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/metric"
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/newrelic/nri-kubernetes/src/definition"
)

// K8sMetricSetTypeGuesser is the metric set type guesser for k8s integrations.
func K8sMetricSetTypeGuesser(_, groupLabel, _ string, _ definition.RawGroups) (string, error) {
	return fmt.Sprintf("K8s%vSample", strings.Title(groupLabel)), nil
}

// K8sClusterMetricsManipulator adds 'clusterName' metric to the MetricSet 'ms',
// taking the value from 'clusterName' argument.
func K8sClusterMetricsManipulator(ms metric.MetricSet, _ sdk.Entity, clusterName string) error {
	return ms.SetMetric("clusterName", clusterName, metric.ATTRIBUTE)
}

// K8sEntityMetricsManipulator adds 'displayName' metric to
// the MetricSet, taking values from entity.name
func K8sEntityMetricsManipulator(ms metric.MetricSet, entity sdk.Entity, _ string) error {
	return ms.SetMetric("displayName", entity.Name, metric.ATTRIBUTE)
}
