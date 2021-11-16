package testutil

import (
	"io"
	"sync"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/integration"
)

// integrationMutex is a mutex used by NewIntegration to instantiate integrations.
// Instantiation cannot run in parallel because the SDK is not thread safe, and will sometimes panic while trying
// to parse flags.
var integrationMutex = sync.Mutex{}

// NewIntegration returns a new integration.Integration ready for use in testing and mocks.
// Integration will use the test name and will fail the test if creation is unsuccessful.
func NewIntegration(t *testing.T) *integration.Integration {
	integrationMutex.Lock()
	defer integrationMutex.Unlock()

	t.Helper()
	intgr, err := integration.New(t.Name(), "test", integration.Writer(io.Discard), integration.InMemoryStore())
	if err != nil {
		t.Fatalf("creating integration: %v", err)
	}

	return intgr
}
