package data

import (
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

// FetchFunc fetches data from a source.
type FetchFunc func() (definition.RawGroups, error)
