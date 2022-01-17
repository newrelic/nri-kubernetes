package grouper

import (
	"fmt"

	"github.com/newrelic/infra-integrations-sdk/log"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/data"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

// Default path is set on the connector
// TODO this should disappear when refactoring the grouper
const defaultPath = ""

type grouper struct {
	queries  []prometheus.Query
	client   client.HTTPGetter
	logger   log.Logger
	entityID string
}

// Group implements Grouper interface by fetching Prometheus metrics from a given component and converting them
// into metrics of a single entity ID, using controlplane Pod name for autodiscovered and Host for external.
func (r *grouper) Group(specGroups definition.SpecGroups) (definition.RawGroups, *data.ErrorGroup) {
	mFamily, err := prometheus.Do(r.client, defaultPath, r.queries)
	if err != nil {
		return nil, &data.ErrorGroup{
			Errors: []error{
				fmt.Errorf("error querying controlplane component %s: %s", r.entityID, err),
			},
		}
	}

	groups, errs := prometheus.GroupEntityMetricsBySpec(specGroups, mFamily, r.entityID)
	if len(errs) > 0 {
		return groups, &data.ErrorGroup{
			Recoverable: true,
			Errors:      errs,
		}
	}

	return groups, nil
}

// New creates a grouper for the given control plane
// component podName.
func New(
	c client.HTTPGetter,
	queries []prometheus.Query,
	logger log.Logger,
	entityID string,
) data.Grouper {
	return &grouper{
		queries:  queries,
		client:   c,
		logger:   logger,
		entityID: entityID,
	}
}
