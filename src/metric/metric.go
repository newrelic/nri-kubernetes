package metric

import (
	"fmt"
	"strings"

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
