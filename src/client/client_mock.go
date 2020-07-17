package client

import (
	"net/http"
	"time"

	"github.com/stretchr/testify/mock"
	"k8s.io/api/core/v1"
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
func (m *MockedKubernetes) FindPodsByLabel(name, value string) (*v1.PodList, error) {
	args := m.Called(name)
	return args.Get(0).(*v1.PodList), args.Error(1)
}

// FindPodByName mocks Kubernetes FindPodByName
func (m *MockedKubernetes) FindPodByName(name string) (*v1.PodList, error) {
	args := m.Called(name)
	return args.Get(0).(*v1.PodList), args.Error(1)
}

// FindPodsByHostname mocks Kubernetes FindPodsByHostname
func (m *MockedKubernetes) FindPodsByHostname(hostname string) (*v1.PodList, error) {
	args := m.Called(hostname)
	return args.Get(0).(*v1.PodList), args.Error(1)
}

// FindServicesByLabel mocks Kubernetes FindServicesByLabel
func (m *MockedKubernetes) FindServicesByLabel(name, value string) (*v1.ServiceList, error) {
	args := m.Called(name, value)
	return args.Get(0).(*v1.ServiceList), args.Error(1)
}
