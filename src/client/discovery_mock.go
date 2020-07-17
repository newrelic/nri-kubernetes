package client

import (
	"net/http"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockDiscoverer is a mock implementation of the Discoverer interface
type MockDiscoverer struct {
	mock.Mock
}

// Discover provides a mock implementation for Discoverer interface
func (m *MockDiscoverer) Discover(timeout time.Duration) (HTTPClient, error) {
	args := m.Called(timeout)
	return args.Get(0).(HTTPClient), args.Error(1)
}

// MockDiscoveredHTTPClient is a mock implementation of the HTTPClient interface
type MockDiscoveredHTTPClient struct {
	mock.Mock
}

// Do provides a mock implementation for HTTPClient interface
func (m *MockDiscoveredHTTPClient) Do(method, path string) (*http.Response, error) {
	args := m.Called(method, path)

	resp := args.Get(0)
	if resp == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// NodeIP provides a mock implementation for HTTPClient interface
func (m *MockDiscoveredHTTPClient) NodeIP() string {
	args := m.Called()
	return args.String(0)
}
