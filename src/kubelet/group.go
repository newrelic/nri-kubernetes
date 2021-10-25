package kubelet

import (
	"fmt"

	"github.com/newrelic/infra-integrations-sdk/log"
	v1 "k8s.io/api/core/v1"

	"github.com/newrelic/nri-kubernetes/v2/src/apiserver"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/data"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric"
)

type kubelet struct {
	apiServer               apiserver.Client
	client                  client.HTTPGetter
	fetchers                []data.FetchFunc
	logger                  log.Logger
	defaultNetworkInterface string
}

// Group implements Grouper interface by fetching RawGroups using both given fetch functions
// and hardcoded fetching calls pulling kubelet summary metrics, node information from Kubernetes API
// and then merging all this information.
func (r *kubelet) Group(definition.SpecGroups) (definition.RawGroups, *data.ErrorGroup) {
	rawGroups := definition.RawGroups{
		"network": {
			"interfaces": definition.RawMetrics{
				"default": r.defaultNetworkInterface,
			},
		},
	}
	for _, f := range r.fetchers {
		g, err := f()
		if err != nil {
			// TODO We don't have to panic when multiple err
			if _, ok := err.(data.ErrorGroup); !ok {
				return nil, &data.ErrorGroup{
					Errors: []error{fmt.Errorf("error querying Kubelet. %s", err)},
				}
			}
		}
		fillGroupsAndMergeNonExistent(rawGroups, g)
	}

	// TODO wrap this process in a new fetchFunc
	response, err := metric.GetMetricsData(r.client)
	if err != nil {
		return nil, &data.ErrorGroup{
			Errors: []error{fmt.Errorf("error querying Kubelet. %s", err)},
		}
	}

	resources, errs := metric.GroupStatsSummary(response)
	if len(errs) != 0 {
		return nil, &data.ErrorGroup{Recoverable: true, Errors: errs}
	}

	fillGroupsAndMergeNonExistent(rawGroups, resources)

	nodeInfo, err := r.apiServer.GetNodeInfo(response.Node.NodeName)
	if err != nil {
		return nil, &data.ErrorGroup{
			Errors: []error{fmt.Errorf("error querying ApiServer: %v", err)},
		}
	}

	var requestedCPUMillis, requestedMemoryBytes int64

	if _, ok := rawGroups["container"]; ok {
		for _, container := range rawGroups["container"] {
			if containerMemoryRequestedBytes, ok := container["memoryRequestedBytes"]; ok {
				// if this map key exist, it's Quantity.MilliValue() (int64)
				requestedMemoryBytes += containerMemoryRequestedBytes.(int64)
			}

			if containerCPURequestedCores, ok := container["cpuRequestedCores"]; ok {
				// if this map key exist, it's Quantity.MilliValue() (int64)
				requestedCPUMillis += containerCPURequestedCores.(int64)
			}
		}
	}

	// Convert node conditions to a map so our ValueFuncs can work nicely with them.
	nodeConditions := make(map[string]int, len(nodeInfo.Conditions))

	for _, condition := range nodeInfo.Conditions {
		conditionValue := -1
		switch condition.Status {
		case v1.ConditionTrue:
			conditionValue = 1
		case v1.ConditionFalse:
			conditionValue = 0
		case v1.ConditionUnknown:
			conditionValue = -1
		default:
			// Should be unreachable as any other value is not allowed by the API. But if it were, we skip it.
			continue
		}

		// Since conditions is a list, there could be duplicate conditions. Check ff we have added this condition before
		// with a different value, and set it to unknown if this is the case.
		if oldValue, ok := nodeConditions[string(condition.Type)]; ok && oldValue != conditionValue {
			conditionValue = -1
		}

		nodeConditions[string(condition.Type)] = conditionValue
	}

	g := definition.RawGroups{
		"node": {
			response.Node.NodeName: definition.RawMetrics{
				"labels":               nodeInfo.Labels,
				"allocatable":          nodeInfo.Allocatable,
				"capacity":             nodeInfo.Capacity,
				"memoryRequestedBytes": requestedMemoryBytes,
				"cpuRequestedCores":    requestedCPUMillis,
				"conditions":           nodeConditions,
				"unschedulable":        nodeInfo.Unschedulable,
				"kubeletVersion":       nodeInfo.KubeletVersion,
			},
		},
	}
	fillGroupsAndMergeNonExistent(rawGroups, g)

	return rawGroups, nil
}

// NewGrouper creates a grouper aware of Kubelet raw metrics.
func NewGrouper(c client.HTTPGetter, logger log.Logger, apiServer apiserver.Client, defaultNetworkInterface string, fetchers ...data.FetchFunc) data.Grouper {
	return &kubelet{
		apiServer:               apiServer,
		client:                  c,
		logger:                  logger,
		fetchers:                fetchers,
		defaultNetworkInterface: defaultNetworkInterface,
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
