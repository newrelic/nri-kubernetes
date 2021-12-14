package controlplane

import (
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

// ComponentName is a typed name for components.
type ComponentName string

const (
	// Scheduler is the Kubernetes Scheduler
	Scheduler ComponentName = "scheduler"
	// Etcd is the Kubernetes etcd
	Etcd ComponentName = "etcd"
	// ControllerManager is the Kubernetes controller manager
	ControllerManager ComponentName = "controller-manager"
	// APIServer is the Kubernetes apiserver
	APIServer ComponentName = "api-server"
)

// Component represents a control plane component from which the
// integration will fetch metrics.
type component struct {
	Name                 ComponentName
	Specs                definition.SpecGroups
	Queries              []prometheus.Query
	AutodiscoverConfigs  []config.AutodiscoverControlPlane
	StaticEndpointConfig *config.Endpoint
}

func newComponents(config config.ControlPlane) []component {
	components := []component{}

	if config.Scheduler.Enabled {
		component := component{
			Name:                 Scheduler,
			Queries:              metric.SchedulerQueries,
			Specs:                metric.SchedulerSpecs,
			StaticEndpointConfig: config.Scheduler.StaticEndpoint,
			AutodiscoverConfigs:  config.Scheduler.Autodiscover,
		}
		components = append(components, component)
	}

	if config.ETCD.Enabled {
		component := component{
			Name:                 Etcd,
			Queries:              metric.EtcdQueries,
			Specs:                metric.EtcdSpecs,
			StaticEndpointConfig: config.ETCD.StaticEndpoint,
			AutodiscoverConfigs:  config.ETCD.Autodiscover,
		}
		components = append(components, component)
	}

	if config.ControllerManager.Enabled {
		component := component{
			Name:                 ControllerManager,
			Queries:              metric.ControllerManagerQueries,
			Specs:                metric.ControllerManagerSpecs,
			StaticEndpointConfig: config.ControllerManager.StaticEndpoint,
			AutodiscoverConfigs:  config.ControllerManager.Autodiscover,
		}
		components = append(components, component)
	}

	if config.APIServer.Enabled {
		component := component{
			Name:                 APIServer,
			Queries:              metric.APIServerQueries,
			Specs:                metric.APIServerSpecs,
			StaticEndpointConfig: config.APIServer.StaticEndpoint,
			AutodiscoverConfigs:  config.APIServer.Autodiscover,
		}
		components = append(components, component)
	}

	return components
}
