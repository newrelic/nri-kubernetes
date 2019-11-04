package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func newBasicHTTPClient(url string) *basicHTTPClient {
	return &basicHTTPClient{
		url: url,
		httpClient: http.Client{
			Timeout: time.Minute * 10, // high for debugging purposes
		},
	}
}

type basicHTTPClient struct {
	url        string
	httpClient http.Client
}

func (b basicHTTPClient) Do(method, path string) (*http.Response, error) {
	endpoint := fmt.Sprintf("%s%s", b.url, path)
	log.Println("Getting: ", endpoint)

	return b.httpClient.Get(endpoint)
}

func (b basicHTTPClient) NodeIP() string {
	return "localhost"
}
