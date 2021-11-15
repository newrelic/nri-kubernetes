package ksm_test

// This file holds the integration tests for the KSM package.

import (
	"fmt"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/integration"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/testutil"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm"
	ksmClient "github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
)

func TestScraper(t *testing.T) {
	for _, version := range testutil.AllVersions() {
		t.Run(fmt.Sprintf("for_version_%s", version), func(t *testing.T) {
			t.Parallel()

			testServer, err := version.Server()
			if err != nil {
				t.Fatalf("Cannot create fake KSM server: %v", err)
			}

			ksmCli, err := ksmClient.New()
			if err != nil {
				t.Fatalf("error creating ksm client: %v", err)
			}

			fakeK8s, err := testutil.FakeK8sClient()
			if err != nil {
				t.Fatalf("Cannot create fake K8s server: %v", err)
			}

			scraper, err := ksm.NewScraper(&config.Mock{
				KSM: config.KSM{
					StaticURL: testServer.KSMEndpoint(),
				},
			}, ksm.Providers{
				K8s: fakeK8s,
				KSM: ksmCli,
			})

			// TODO: WIP
			i, err := integration.New("nri-kubernetes-ksm-test", "0.0.0")
			if err != nil {
				t.Fatal(err)
			}

			err = scraper.Run(i)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
