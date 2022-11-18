package definition

import (
	"fmt"
	"strings"
)

// K8sMetricSetTypeGuesser is the metric set type guesser for k8s integrations.
func K8sMetricSetTypeGuesser(_, groupLabel, _ string, _ RawGroups) (string, error) {
	var sampleName string
	for _, s := range strings.Split(groupLabel, "-") {
		sampleName += strings.Title(s) //nolint: staticcheck // TODO: use golang.org/x/text/cases assuring no breaking changes.
	}
	return fmt.Sprintf("K8s%vSample", sampleName), nil
}
