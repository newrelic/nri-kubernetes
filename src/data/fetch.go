package data

import (
	"github.com/newrelic/nri-kubernetes/src/definition"
)

// FetchFunc fetches data from a source.
type FetchFunc func() (definition.RawGroups, error)
