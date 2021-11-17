package testutil

import (
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const servicesFile = "services.yaml"
const namespacesFile = "namespaces.yaml"

func K8sEverything() []runtime.Object {
	return []runtime.Object{
		K8sNamespaces(),
		K8sServices(),
	}
}

func K8sNamespaces() runtime.Object {
	var namespaceList corev1.NamespaceList
	if err := loadYaml(&namespaceList, namespacesFile); err != nil {
		panic(err)
	}

	return &namespaceList
}

func K8sServices() runtime.Object {
	var services corev1.ServiceList
	if err := loadYaml(&services, servicesFile); err != nil {
		panic(err)
	}

	return &services
}

func loadYaml(dst interface{}, path string) error {
	yamlFile, err := testDataDir.ReadFile(filepath.Join(testDataRootDir, path))
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
