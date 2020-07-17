package kubelet

import (
	"fmt"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/data"
	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/newrelic/nri-kubernetes/src/kubelet/metric"
	"github.com/sirupsen/logrus"
)

type kubelet struct {
	client   client.HTTPClient
	fetchers []data.FetchFunc
	logger   *logrus.Logger
}

func (r *kubelet) Group(definition.SpecGroups) (definition.RawGroups, *data.ErrorGroup) {
	rawGroups := make(definition.RawGroups)
	for _, f := range r.fetchers {
		g, err := f()
		if err != nil {
			// TODO We don't have to panic when multiple err
			if _, ok := err.(data.ErrorGroup); !ok {
				return nil, &data.ErrorGroup{
					Recoverable: false,
					Errors:      []error{fmt.Errorf("error querying Kubelet. %s", err)},
				}
			}

		}
		fillGroupsAndMergeNonExistent(rawGroups, g)
	}

	// TODO wrap this process in a new fetchFunc
	response, err := metric.GetMetricsData(r.client)
	if err != nil {
		return nil, &data.ErrorGroup{
			Recoverable: false,
			Errors:      []error{fmt.Errorf("error querying Kubelet. %s", err)},
		}
	}

	resources, errs := metric.GroupStatsSummary(response)
	if len(errs) != 0 {
		return nil, &data.ErrorGroup{Recoverable: true, Errors: errs}
	}

	fillGroupsAndMergeNonExistent(rawGroups, resources)

	return rawGroups, nil
}

// NewGrouper creates a grouper aware of Kubelet raw metrics.
func NewGrouper(c client.HTTPClient, logger *logrus.Logger, fetchers ...data.FetchFunc) data.Grouper {
	return &kubelet{
		client:   c,
		logger:   logger,
		fetchers: fetchers,
	}
}

func fillGroupsAndMergeNonExistent(destination definition.RawGroups, from definition.RawGroups) {
	for l, g := range from {
		if _, ok := destination[l]; !ok {
			destination[l] = g
			continue
		}

		for entityID, e := range destination[l] {
			if _, ok := g[entityID]; !ok {
				continue
			}

			for k, v := range g[entityID] {
				if _, ok := e[k]; !ok {
					e[k] = v
				}
			}
		}
	}
}
