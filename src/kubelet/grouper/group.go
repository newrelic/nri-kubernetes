package grouper

import (
	"fmt"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	listersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/data"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/kubelet/metric"
)

type grouper struct {
	Config
	logger *log.Logger
}

type Config struct {
	NodeGetter              listersv1.NodeLister
	Client                  client.HTTPGetter
	Fetchers                []data.FetchFunc
	DefaultNetworkInterface string
}

type OptionFunc func(kc *grouper) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger *log.Logger) OptionFunc {
	return func(kc *grouper) error {
		kc.logger = logger
		return nil
	}
}

// New returns a data.Grouper that groups Kubelet metrics.
func New(config Config, opts ...OptionFunc) (data.Grouper, error) {
	if config.NodeGetter == nil {
		return nil, fmt.Errorf("NodeGetter must be set")
	}

	g := &grouper{
		Config: config,
		logger: logutil.Discard,
	}

	for i, opt := range opts {
		if err := opt(g); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	return g, nil
}

// Group implements Grouper interface by fetching RawGroups using both given fetch functions
// and hardcoded fetching calls pulling kubelet summary metrics, node information from Kubernetes API
// and then merging all this information.
func (r *grouper) Group(definition.SpecGroups) (definition.RawGroups, *data.ErrorGroup) {
	rawGroups := definition.RawGroups{
		"network": {
			"interfaces": definition.RawMetrics{
				"default": r.DefaultNetworkInterface,
			},
		},
	}
	for _, f := range r.Fetchers {
		g, err := f()
		if err != nil {
			if _, ok := err.(data.ErrorGroup); !ok {
				return nil, &data.ErrorGroup{
					Errors: []error{fmt.Errorf("error querying Kubelet. %s", err)},
				}
			}
		}
		fillGroupsAndMergeNonExistent(rawGroups, g)
	}

	// TODO wrap this process in a new fetchFunc
	response, err := metric.GetMetricsData(r.Client)
	if err != nil {
		return nil, &data.ErrorGroup{
			Errors: []error{fmt.Errorf("error querying Kubelet. %s", err)},
		}
	}

	resources, errs := metric.GroupStatsSummary(response)
	if len(errs) > 0 {
		return nil, &data.ErrorGroup{
			Recoverable: true,
			Errors:      errs,
		}
	}

	fillGroupsAndMergeNonExistent(rawGroups, resources)

	node, err := r.NodeGetter.Get(response.Node.NodeName)
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
	nodeConditions := make(map[string]int, len(node.Status.Conditions))

	for _, condition := range node.Status.Conditions {
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

	runningPodsCount := r.countRunningPods(rawGroups)

	g := definition.RawGroups{
		"node": {
			response.Node.NodeName: definition.RawMetrics{
				"labels":               node.Labels,
				"allocatable":          node.Status.Allocatable,
				"capacity":             node.Status.Capacity,
				"memoryRequestedBytes": requestedMemoryBytes,
				"cpuRequestedCores":    requestedCPUMillis,
				"conditions":           nodeConditions,
				"unschedulable":        node.Spec.Unschedulable,
				"kubeletVersion":       node.Status.NodeInfo.KubeletVersion,
				"runningPods":          runningPodsCount,
			},
		},
	}
	fillGroupsAndMergeNonExistent(rawGroups, g)

	return rawGroups, nil
}

// Count the number of pods in a 'Running' state for the current node.
func (r *grouper) countRunningPods(rawGroups definition.RawGroups) int {
	runningPodsCount := 0
	if pods, ok := rawGroups["pod"]; ok {
		for _, podMetrics := range pods {
			// The pod data is already scoped to the current node by the fetcher,
			// so we only need to check the status.
			if status, ok := podMetrics["status"].(string); ok && status == "Running" {
				runningPodsCount++
			}
		}
	}
	return runningPodsCount
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
