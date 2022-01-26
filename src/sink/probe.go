package sink

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

const backoff = 3 * time.Second

var ErrProbeTimeout = errors.New("probe did not succeed")

func WaitForEndpoint(endpoint string, timeout time.Duration) error {
	start := time.Now()

	client := http.DefaultClient

	for {
		time.Sleep(backoff)

		if time.Since(start) > timeout {
			return fmt.Errorf("%w after %v", ErrProbeTimeout, timeout)
		}

		resp, err := client.Get(endpoint)
		if err != nil {
			continue
		}

		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		return nil
	}
}
