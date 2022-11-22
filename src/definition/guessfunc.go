package definition

import (
	"fmt"
	"strings"
)

// K8sMetricSetTypeGuesser is the metric set type guesser for k8s integrations.
// It composes the 'event_type' value using the group label defined in the corresponding specs.
func K8sMetricSetTypeGuesser(groupLabel string) (string, error) {
	var sampleName string
	for _, s := range strings.Split(groupLabel, "-") {
		sampleName += strings.Title(s) //nolint: staticcheck // TODO: use golang.org/x/text/cases assuring no breaking changes.
	}
	return fmt.Sprintf("K8s%vSample", sampleName), nil
}
