package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
)

func TestSetupKubelet(t *testing.T) {
	c := config.Config{
		Verbose: false,
		Kubelet: config.Kubelet{},
		NamespaceSelector: &config.NamespaceSelector{
			MatchLabels: map[string]string{
				"newrelic.com/scrape": "true",
			},
		},
	}
	logger = logutil.Discard
	providers := clusterClients{
		k8s: fake.NewSimpleClientset(),
	}
	scraper, err := setupKSM(&c, &providers)
	assert.NoError(t, err)
	assert.NotEmpty(t, scraper)
}

func TestSetupKSM(t *testing.T) {
	c := config.Config{
		Verbose: false,
		KSM:     config.KSM{},
		NamespaceSelector: &config.NamespaceSelector{
			MatchLabels: map[string]string{
				"newrelic.com/scrape": "true",
			},
		},
	}
	logger = logutil.Discard
	providers := clusterClients{
		k8s: fake.NewSimpleClientset(),
	}
	scraper, err := setupKSM(&c, &providers)
	assert.NoError(t, err)
	assert.NotEmpty(t, scraper)
}
