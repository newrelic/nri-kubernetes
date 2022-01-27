package discovery_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
)

type fakeDiscoverer func() ([]string, error)

func (fd fakeDiscoverer) Discover() ([]string, error) {
	return fd()
}

func succeedDiscoverAfter(attempts int) fakeDiscoverer {
	current := 0
	return func() ([]string, error) {
		if current >= attempts {
			return []string{"success"}, nil
		}

		current++

		return nil, nil
	}
}

func Test_edt_forwards_errors(t *testing.T) {
	t.Parallel()

	innerErr := errors.New("inner error")
	timeouter := discovery.EndpointsDiscovererWithTimeout{
		EndpointsDiscoverer: fakeDiscoverer(func() ([]string, error) {
			return nil, innerErr
		}),
		BackoffDelay: 0,
		Timeout:      1 * time.Second,
	}

	_, err := timeouter.Discover()
	if !errors.Is(err, innerErr) {
		t.Fatalf("unexpected error %v returned", err)
	}
}

func Test_edt_forwards_endpoints(t *testing.T) {
	t.Parallel()

	innerList := []string{"foobar"}

	timeouter := discovery.EndpointsDiscovererWithTimeout{
		EndpointsDiscoverer: fakeDiscoverer(func() ([]string, error) {
			return innerList, nil
		}),
		BackoffDelay: 0,
		Timeout:      1 * time.Second,
	}

	list, err := timeouter.Discover()
	if err != nil {
		t.Fatal("error should have been nil")
	}

	if !reflect.DeepEqual(innerList, list) {
		t.Fatal("returned list is not equal")
	}
}

func Test_edt(t *testing.T) {
	t.Parallel()

	type testEntry struct {
		name        string
		fd          fakeDiscoverer
		wait        time.Duration
		timeout     time.Duration
		expectedErr error
	}

	for _, entry := range []testEntry{
		{
			name:        "returns_at_once",
			timeout:     2 * time.Second,
			fd:          succeedDiscoverAfter(0),
			expectedErr: nil,
		},
		{
			name:        "returns_within_threshold",
			wait:        1 * time.Second,
			timeout:     4 * time.Second,
			fd:          succeedDiscoverAfter(3),
			expectedErr: nil,
		},
		{
			name:        "fails_not_in_threshold",
			wait:        1 * time.Second,
			timeout:     2 * time.Second,
			fd:          succeedDiscoverAfter(3),
			expectedErr: discovery.ErrDiscoveryTimeout,
		},
	} {
		entry := entry

		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			timeouter := discovery.EndpointsDiscovererWithTimeout{
				EndpointsDiscoverer: entry.fd,
				BackoffDelay:        entry.wait,
				Timeout:             entry.timeout,
			}

			_, err := timeouter.Discover()
			if err == nil && entry.expectedErr != nil {
				t.Fatal("should have errored")
			}

			if !errors.Is(err, entry.expectedErr) {
				t.Fatalf("error is not the expected type: %v", err)
			}
		})
	}
}
