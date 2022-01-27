package controlplane

import (
	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/metric"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
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

// secretNamespaces returns all namespaces where secrets are store.
func secretNamespaces(components []component) (namespaces []string) {
	s := make(map[string]struct{})

	for _, c := range components {
		if c.StaticEndpointConfig != nil {
			s[secretNamespace(c.StaticEndpointConfig.Auth)] = struct{}{}
		}

		for _, autodiscover := range c.AutodiscoverConfigs {
			for _, endpoint := range autodiscover.Endpoints {
				s[secretNamespace(endpoint.Auth)] = struct{}{}
			}
		}
	}

	for n := range s {
		if n != "" {
			namespaces = append(namespaces, n)
		}
	}

	return
}

func secretNamespace(auth *config.Auth) string {
	if auth == nil || auth.MTLS == nil {
		return ""
	}

	return auth.MTLS.TLSSecretNamespace
}

func autodiscoverNamespaces(components []component) (namespaces []string) {
	for _, c := range components {
		for _, a := range c.AutodiscoverConfigs {
			if a.Namespace != "" {
				namespaces = append(namespaces, a.Namespace)
			}
		}
	}

	return
}
