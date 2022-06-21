package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

	"time"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
)

func TestSetupKubelet(t *testing.T) {
	c := config.Config{
		Verbose: false,
		Kubelet: config.Kubelet{},
		NamespaceSelector: &config.NamespaceSelector{
			MatchLabels: map[string]interface{}{
				"newrelic.com/scrape": "true",
			},
		},
		Interval: 10 * time.Second,
	}
	logger = logutil.Discard
	namespaceCache := discovery.NewNamespaceInMemoryStore(logger)
	providers := clusterClients{
		k8s: fake.NewSimpleClientset(),
	}
	scraper, err := setupKSM(&c, &providers, namespaceCache)
	assert.NoError(t, err)
	assert.NotEmpty(t, scraper)
	assert.NotEmpty(t, scraper.Filterer)
}

func TestSetupKSM(t *testing.T) {
	c := config.Config{
		Verbose: false,
		KSM:     config.KSM{},
		NamespaceSelector: &config.NamespaceSelector{
			MatchLabels: map[string]interface{}{
				"newrelic.com/scrape": "true",
			},
		},
		Interval: 10 * time.Second,
	}
	logger = logutil.Discard
	namespaceCache := discovery.NewNamespaceInMemoryStore(logger)
	providers := clusterClients{
		k8s: fake.NewSimpleClientset(),
	}
	scraper, err := setupKSM(&c, &providers, namespaceCache)
	assert.NoError(t, err)
	assert.NotEmpty(t, scraper)
	assert.NotEmpty(t, scraper.Filterer)
}
