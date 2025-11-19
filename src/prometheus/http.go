package prometheus

import (
	"net/http"
)

// AcceptHeader starting with ksm 1.5 only plain text encoding is supported.
const AcceptHeader = `text/plain`

// NewRequest returns a new Request given a method, URL, setting the required header for accepting protobuf.
func NewRequest(url string) (*http.Request, error) {
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	r.Header.Set("Accept", AcceptHeader)

	return r, nil
}
