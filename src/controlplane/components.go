package controlplane

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/newrelic/nri-kubernetes/src/metric"

	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
)

// Component represents a control plane component from which the
// integration will fetch metrics.
type Component struct {
	Skip                            bool
	SkipReason                      string
	Name                            ComponentName
	LabelValue                      string
	TLSSecretName                   string
	TLSSecretNamespace              string
	Endpoint                        url.URL
	UseServiceAccountAuthentication bool
	UseMTLSAuthentication           bool
	Specs                           definition.SpecGroups
	Queries                         []prometheus.Query
	Labels                          []labels
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
		etcd.UseMTLSAuthentication = true
	}
}

// WithAPIServerSecurePort configures the API Server component to be query using HTTPS, with the Service Account token
// as authentication
func WithAPIServerSecurePort(port string) ComponentOption {
	return func(components []Component) {
		apiServer := findComponentByName(APIServer, components)
		if apiServer == nil {
			panic(fmt.Sprintf("expected component %s in list of components, but not found", string(APIServer)))
		}

		apiServer.UseServiceAccountAuthentication = true
		apiServer.Endpoint = url.URL{
			Scheme: "https",
			Host:   fmt.Sprintf("localhost:%s", port),
		}
	}
}

// WithEndpointURL configures the component to be use a specific endpoint URL and enables
// Service Account token as authentication
func WithEndpointURL(name ComponentName, endpointURL string) ComponentOption {
	return func(components []Component) {
		component := findComponentByName(name, components)
		if component == nil {
			panic(fmt.Sprintf("expected component %s in list of components, but not found", string(name)))
		}

		url, err := url.Parse(endpointURL)
		if err != nil {
			panic(fmt.Sprintf("Endpoint URL %s for component %s is not a valid URL", endpointURL, string(name)))
		}

		component.UseServiceAccountAuthentication = (strings.ToLower(url.Scheme) == "https")
		component.Endpoint = *url
	}
}

// findComponentByName will find the component with the given name
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
				// Kops / Kubeadm / ClusterAPI
				{"k8s-app": "kube-scheduler"},
				{"tier": "control-plane", "component": "kube-scheduler"},
				// OpenShift
				{"app": "openshift-kube-scheduler", "scheduler": "true"},
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
				// Kops / Kubeadm / ClusterAPI
				{"k8s-app": "etcd-manager-main"},
				{"tier": "control-plane", "component": "etcd"},
				// OpenShift
				{"k8s-app": "etcd"},
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
				// Kops / Kubeadm / ClusterAPI
				{"k8s-app": "kube-controller-manager"},
				{"tier": "control-plane", "component": "kube-controller-manager"},
				// OpenShift
				{"app": "kube-controller-manager", "kube-controller-manager": "true"},
				{"app": "controller-manager", "controller-manager": "true"},
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
				// Kops / Kubeadm / ClusterAPI
				{"k8s-app": "kube-apiserver"},
				{"tier": "control-plane", "component": "kube-apiserver"},
				// OpenShift
				{"app": "openshift-kube-apiserver", "apiserver": "true"},
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
