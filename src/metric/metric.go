package metric

import (
	"fmt"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"

	"github.com/newrelic/nri-kubernetes/v2/src/definition"
)

// K8sMetricSetTypeGuesser is the metric set type guesser for k8s integrations.
func K8sMetricSetTypeGuesser(_, groupLabel, _ string, _ definition.RawGroups) (string, error) {
	var sampleName string
	for _, s := range strings.Split(groupLabel, "-") {
		sampleName += strings.Title(s)
	}
	return fmt.Sprintf("K8s%vSample", sampleName), nil
}

// K8sClusterMetricsManipulator adds 'clusterName' metric to the MetricSet 'ms',
// taking the value from 'clusterName' argument.
func K8sClusterMetricsManipulator(ms *metric.Set, _ *integration.EntityMetadata, clusterName string) error {
	return ms.SetMetric("clusterName", clusterName, metric.ATTRIBUTE)
}

// K8sEntityMetricsManipulator adds 'displayName' metric to
// the MetricSet, taking values from entity.name
func K8sEntityMetricsManipulator(ms *metric.Set, entity *integration.EntityMetadata, _ string) error {
	return ms.SetMetric("displayName", entity.Name, metric.ATTRIBUTE)
}
