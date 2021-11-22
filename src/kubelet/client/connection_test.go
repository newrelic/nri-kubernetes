package client

import (
	"k8s.io/client-go/rest"
	"testing"
)

func TestCheckConnection(t *testing.T) {
	config, _ := rest.InClusterConfig()
	fake.in
	rest.TransportFor(config)
}
