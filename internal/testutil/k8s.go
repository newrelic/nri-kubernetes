package testutil

import (
	"context"
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const servicesFile = "services.yml"

func FakeK8sClient() (*fake.Clientset, error) {
	fakeCs := fake.NewSimpleClientset()

	k8sServicesYaml, err := testDataDir.ReadFile(filepath.Join(testDataRootDir, servicesFile))
	if err != nil {
		return nil, fmt.Errorf("reading testdata services.yml: %w", err)
	}

	var services []*corev1.Service
	err = yaml.Unmarshal(k8sServicesYaml, &services)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling testata/services.yml: %w", err)
	}

	// KSM scraper relies on a list of services to add extra data to KSM metrics
	for _, svc := range services {
		err := k8sCreateNamespaceIfMissing(fakeCs, svc.Namespace)
		if err != nil {
			return nil, fmt.Errorf("creating ns %q for service %q: %w", svc.Namespace, svc.Name, err)
		}

		_, err = fakeCs.CoreV1().Services(svc.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating service %q: %w", svc.Name, err)
		}
	}

	// TODO: Populate more objects as needed:

	return fakeCs, nil
}

func k8sCreateNamespaceIfMissing(k8s kubernetes.Interface, namespace string) error {
	if namespace == "" {
		return nil
	}

	_, err := k8s.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	_, err = k8s.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})
	return err
}
