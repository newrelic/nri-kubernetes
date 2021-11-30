package testutil

import (
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	endpointsFile  = "endpoints.yaml"
	namespacesFile = "namespaces.yaml"
	nodesFile      = "nodes.yaml"
	podsFile       = "pods.yaml"
	servicesFile   = "services.yaml"
)

type K8s struct {
	version Version
}

func newK8s(v Version) (K8s, error) {
	_, err := testDataDir.ReadDir(filepath.Join(testDataRootDir, string(v)))
	if err != nil {
		return K8s{}, fmt.Errorf("cannot stat testdata dir for version %q: %w", string(v), err)
	}

	return K8s{v}, nil
}

func (k K8s) Everything() []runtime.Object {
	return []runtime.Object{
		k.Endpoints(),
		k.Namespaces(),
		k.Nodes(),
		k.Pods(),
		k.Services(),
	}
}

func (k K8s) Namespaces() runtime.Object {
	var namespaceList corev1.NamespaceList
	if err := k.loadYaml(&namespaceList, namespacesFile); err != nil {
		panic(err)
	}

	return &namespaceList
}

func (k K8s) Services() runtime.Object {
	var services corev1.ServiceList
	if err := k.loadYaml(&services, servicesFile); err != nil {
		panic(err)
	}

	return &services
}

func (k K8s) Nodes() runtime.Object {
	var nodes corev1.NodeList
	if err := k.loadYaml(&nodes, nodesFile); err != nil {
		panic(err)
	}

	return &nodes
}

func (k K8s) Endpoints() runtime.Object {
	var nodes corev1.EndpointsList
	if err := k.loadYaml(&nodes, endpointsFile); err != nil {
		panic(err)
	}

	return &nodes
}

func (k K8s) Pods() runtime.Object {
	var nodes corev1.PodList
	if err := k.loadYaml(&nodes, podsFile); err != nil {
		panic(err)
	}

	return &nodes
}

func (k K8s) loadYaml(dst interface{}, path string) error {
	yamlFile, err := testDataDir.ReadFile(filepath.Join(testDataRootDir, string(k.version), path))
	if err != nil {
		return fmt.Errorf("reading testdata services.yaml: %w", err)
	}

	var services corev1.ServiceList
	err = yaml.Unmarshal(yamlFile, &services)
	if err != nil {
		return fmt.Errorf("unmarshalling testata/services.yaml: %w", err)
	}

	return yaml.Unmarshal(yamlFile, dst)
}
