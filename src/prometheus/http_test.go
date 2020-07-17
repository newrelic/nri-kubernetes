package prometheus

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRequest(t *testing.T) {
	r, err := NewRequest(http.MethodGet, "http://example.com")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, AcceptHeader, r.Header.Get("Accept"))
	assert.Equal(t, "http://example.com", r.URL.String())
	assert.Equal(t, http.MethodGet, r.Method)
}
