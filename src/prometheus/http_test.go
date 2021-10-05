package prometheus

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequest(t *testing.T) {
	r, err := NewRequest(http.MethodGet, "http://example.com")
	require.NoError(t, err)

	assert.Equal(t, AcceptHeader, r.Header.Get("Accept"))
	assert.Equal(t, "http://example.com", r.URL.String())
	assert.Equal(t, http.MethodGet, r.Method)
}
