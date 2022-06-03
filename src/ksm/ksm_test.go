package ksm_test

// This file holds the integration tests for the KSM package.

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/testutil"
	"github.com/newrelic/nri-kubernetes/v3/internal/testutil/asserter"
	"github.com/newrelic/nri-kubernetes/v3/internal/testutil/asserter/exclude"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/ksm"
	ksmClient "github.com/newrelic/nri-kubernetes/v3/src/ksm/client"
	"github.com/newrelic/nri-kubernetes/v3/src/metric"
	"github.com/stretchr/testify/assert"
)

type NamespaceFilterMock struct{}

func (nf NamespaceFilterMock) IsAllowed(namespace string) bool {
	if namespace == "scraper" {
		return false
	}
	return true
}

func TestScraper(t *testing.T) {
	// Create an asserter with the settings that are shared for all test scenarios.
	asserter := asserter.New().
		Using(metric.KSMSpecs).
		Excluding(
			// Exclude service.loadBalancerIP unless service is e2e-lb (specially crafted to have a fake one)
			func(group string, spec *definition.Spec, ent *integration.Entity) bool {
				return group == "service" && spec.Name == "loadBalancerIP" && ent.Metadata.Name != "e2e-lb"
			},
			// The following HPA metrics operate in a true-or-NULL basis, and there won't be present if condition is
			// false.
			exclude.Exclude(
				exclude.Groups("hpa"),
				exclude.Metrics("isActive", "isAble", "isLimited"),
			),
		)

	for _, v := range testutil.AllVersions() {
		// Make a copy of the version variable to use it concurrently
		version := v
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

			k8sData, err := version.K8s()
			if err != nil {
				t.Fatalf("error instantiating fake k8s objects: %v", err)
			}

			fakeK8s := fake.NewSimpleClientset(k8sData.Everything()...)
			scraper, err := ksm.NewScraper(&config.Config{
				KSM: config.KSM{
					StaticURL: testServer.KSMEndpoint(),
				},
				ClusterName: t.Name(),
			}, ksm.Providers{
				K8s: fakeK8s,
				KSM: ksmCli,
			})

			require.NoError(t, err)

			i := testutil.NewIntegration(t)

			err = scraper.Run(i)
			if err != nil {
				t.Fatalf("running scraper: %v", err)
			}

			// Call the asserter for the entities of this particular sub-test.
			asserter.On(i.Entities).Assert(t)
		})
	}
}

func TestScraper_FilterNamespace(t *testing.T) {
	version := testutil.LatestVersion()
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

		k8sData, err := version.K8s()
		if err != nil {
			t.Fatalf("error instantiating fake k8s objects: %v", err)
		}

		fakeK8s := fake.NewSimpleClientset(k8sData.Everything()...)
		scraper, err := ksm.NewScraper(
			&config.Config{
				KSM: config.KSM{
					StaticURL: testServer.KSMEndpoint(),
				},
				ClusterName: t.Name(),
			}, ksm.Providers{
				K8s: fakeK8s,
				KSM: ksmCli,
			},
			ksm.WithFilterer(NamespaceFilterMock{}),
		)

		require.NoError(t, err)

		i := testutil.NewIntegration(t)

		err = scraper.Run(i)
		if err != nil {
			t.Fatalf("running scraper: %v", err)
		}

		assert.Equal(t, 18, len(i.Entities))
	})
}
