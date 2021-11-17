package client

import (
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
)

// MockedKubernetes is a Mock for the Kubernetes interface to be used only in tests
type MockedKubernetes struct {
	mock.Mock
}

// ServerVersion mocks Kubernetes ServerVersion
func (m *MockedKubernetes) ServerVersion() (*version.Info, error) {
	args := m.Called()
	return args.Get(0).(*version.Info), args.Error(1)
}

// FindSecret mocks Kubernetes FindSecret
func (m *MockedKubernetes) FindSecret(name, namespace string) (*v1.Secret, error) {
	args := m.Called(name)
	return args.Get(0).(*v1.Secret), args.Error(1)
}

// GetClient mocks Kubernetes GetClient
func (m *MockedKubernetes) GetClient() *kubernetes.Clientset {
	return nil
}
