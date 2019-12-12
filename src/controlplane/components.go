package controlplane

import (
	"fmt"
	"net/url"

	"github.com/newrelic/nri-kubernetes/src/metric"

	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
)

// Component represents a control plane component from which the
// integration will fetch metrics.
type Component struct {
	Skip               bool
	SkipReason         string
	Name               ComponentName
	LabelValue         string
	TLSSecretName      string
	TLSSecretNamespace string
	Endpoint           url.URL
	Specs              definition.SpecGroups
	Queries            []prometheus.Query
	Labels             []labels
}

// ComponentName is a typed name for components
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

// ComponentOption configures the list of components
type ComponentOption func([]Component)

// WithEtcdTLSConfig configures the etcd component to use (M)TLS using credentials stored in a secret.
// The secret should contains the following fields:
// "cert": the client certificate
// "key": the client's private key
// "cacert": optional, the cacert of the ETCD server. If omitted, insecureSkipVerify should be set to "true"
// "insecureSkipVerify": optional, if set to "true", ETCD's server certificate will not be verified
func WithEtcdTLSConfig(etcdTLSSecretName, etcdTLSSecretNamespace string) ComponentOption {
	return func(components []Component) {
		etcd := findComponentByName(Etcd, components)
		if etcd == nil {
			panic(fmt.Sprintf("expected component %s in list of components, but not found", string(Etcd)))
		}

		etcd.TLSSecretName = etcdTLSSecretName
		etcd.TLSSecretNamespace = etcdTLSSecretNamespace
	}
}

// findComponentByName will find the compeont with the given name
func findComponentByName(name ComponentName, components []Component) *Component {
	for i := range components {
		if components[i].Name == name {
			return &components[i]
		}
	}
	return nil
}

// labels is a collection of labels, key-value style
type labels map[string]string

// BuildComponentList returns a list of components that the integration will monitor.
func BuildComponentList(options ...ComponentOption) []Component {
	components := []Component{
		{
			Name: Scheduler,
			Labels: []labels{
				{"k8s-app": "kube-scheduler"},
				{"tier": "control-plane", "component": "kube-scheduler"},
			},
			Queries: metric.SchedulerQueries,
			Specs:   metric.SchedulerSpecs,
			Endpoint: url.URL{
				Scheme: "http",
				Host:   "localhost:10251",
			},
		},
		{
			Name: Etcd,
			Labels: []labels{
				{"k8s-app": "etcd-manager-main"},
				{"tier": "control-plane", "component": "etcd"},
			},
			Queries: metric.EtcdQueries,
			Specs:   metric.EtcdSpecs,
			Endpoint: url.URL{
				Scheme: "https",
				Host:   "127.0.0.1:4001",
			},
		},
		{
			Name: ControllerManager,
			Labels: []labels{
				{"k8s-app": "kube-controller-manager"},
				{"tier": "control-plane", "component": "kube-controller-manager"},
			},
			Queries: metric.ControllerManagerQueries,
			Specs:   metric.ControllerManagerSpecs,
			Endpoint: url.URL{
				Scheme: "http",
				Host:   "localhost:10252",
			},
		},
		{
			Name: APIServer,
			Labels: []labels{
				{"k8s-app": "kube-apiserver"},
				{"tier": "control-plane", "component": "kube-apiserver"},
			},
			Queries: metric.APIServerQueries,
			Specs:   metric.APIServerSpecs,
			Endpoint: url.URL{
				Scheme: "http",
				Host:   "localhost:8080",
			},
		},
	}

	for _, opt := range options {
		opt(components)
	}

	validateComponentConfigurations(components)

	return components
}

// validateComponentConfiguration will check if the components are properly configured.
// If they are not, they will be skipped.
func validateComponentConfigurations(components []Component) {
	etcd := findComponentByName(Etcd, components)
	if etcd.TLSSecretName == "" {
		etcd.Skip = true
		etcd.SkipReason = "etcd requires TLS configuration, none given"
	}
}
