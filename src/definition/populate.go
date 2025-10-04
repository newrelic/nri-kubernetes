package definition

import (
	"fmt"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
)

const (
	NamespaceGroup         = "namespace"
	NamespaceFilteredLabel = "nrFiltered"
)

// GuessFunc guesses from data.
type GuessFunc func(groupLabel string) (string, error)

type IntegrationPopulateConfig struct {
	Integration   *integration.Integration
	ClusterName   string
	K8sVersion    fmt.Stringer
	MsTypeGuesser GuessFunc
	Groups        RawGroups
	Specs         SpecGroups
	Filterer      discovery.NamespaceFilterer
}
