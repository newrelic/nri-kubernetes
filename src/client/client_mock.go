package client

import (
	"net/http"
	"time"

	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// MockedKubernetes is a Mock for the Kubernetes interface to be used only in tests
type MockedKubernetes struct {
	mock.Mock
}

// Config mocks Kubernetes Config
func (m *MockedKubernetes) Config() *rest.Config {
	args := m.Called()
	return args.Get(0).(*rest.Config)
}

// ServerVersion mocks Kubernetes ServerVersion
func (m *MockedKubernetes) ServerVersion() (*version.Info, error) {
	args := m.Called()
	return args.Get(0).(*version.Info), args.Error(1)
}

// SecureHTTPClient mocks Kubernetes SecureHTTPClient
func (m *MockedKubernetes) SecureHTTPClient(timeout time.Duration) (*http.Client, error) {
	args := m.Called(timeout)
	return args.Get(0).(*http.Client), args.Error(1)
}

// FindNode mocks Kubernetes FindNode
func (m *MockedKubernetes) FindNode(name string) (*v1.Node, error) {
	args := m.Called(name)
	return args.Get(0).(*v1.Node), args.Error(1)
}

// FindPodsByLabel mocks Kubernetes FindPodsByLabel
func (m *MockedKubernetes) FindPodsByLabel(namespace string, labelSelector metav1.LabelSelector) (*v1.PodList, error) {
	args := m.Called(namespace, labelSelector)
	return args.Get(0).(*v1.PodList), args.Error(1)
}

// FindServicesByLabel mocks Kubernetes FindServicesByLabel
func (m *MockedKubernetes) FindServicesByLabel(namespace string, labelSelector metav1.LabelSelector) (*v1.ServiceList, error) {
	args := m.Called(namespace, labelSelector)
	return args.Get(0).(*v1.ServiceList), args.Error(1)
}

// FindSecret mocks Kubernetes FindSecret
func (m *MockedKubernetes) FindSecret(name, namespace string) (*v1.Secret, error) {
	args := m.Called(name)
	return args.Get(0).(*v1.Secret), args.Error(1)
}

// ListServices mocks Kubernetes ListServices
func (m *MockedKubernetes) ListServices(namespace string) (*v1.ServiceList, error) {
	args := m.Called()
	return args.Get(0).(*v1.ServiceList), args.Error(1)
}

// GetClient mocks Kubernetes GetClient
func (m *MockedKubernetes) GetClient() *kubernetes.Clientset {
	return nil
}
