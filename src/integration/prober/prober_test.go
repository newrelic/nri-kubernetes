package prober_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/v3/src/integration/prober"
)

func succeedAfter(duration time.Duration) http.HandlerFunc {
	creation := time.Now()
	return func(rw http.ResponseWriter, request *http.Request) {
		if time.Since(creation) > duration {
			rw.WriteHeader(http.StatusOK)
			return
		}

		rw.WriteHeader(http.StatusInternalServerError)
	}
}

func TestProber_fails_as_expected(t *testing.T) {
	t.Parallel()

	p, err := prober.New(4*time.Second, 300*time.Millisecond)
	if err != nil {
		t.Fatalf("Error building prober: %v", err)
	}

	server := httptest.NewServer(succeedAfter(5 * time.Second))

	err = p.Probe(server.URL)
	if !errors.Is(err, prober.ErrProbeTimeout) {
		t.Fatalf("Expected timeout error, got %v", err)
	}
}

func TestProber_succeeds(t *testing.T) {
	t.Parallel()

	p, err := prober.New(15*time.Second, 300*time.Millisecond)
	if err != nil {
		t.Fatalf("Error building prober: %v", err)
	}

	server := httptest.NewServer(succeedAfter(5 * time.Second))

	err = p.Probe(server.URL)
	if errors.Is(err, prober.ErrProbeTimeout) {
		t.Fatalf("Expected timeout error, got %v", err)
	}
}
