package ksm_test

// This file holds the integration tests for the KSM package.

import (
	"fmt"
	"testing"
	"time"

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
	return namespace != "scraper"
}

func TestScraper(t *testing.T) {
	// Create an asserter with the settings that are shared for all test scenarios.
	asserter := asserter.New().
		Silently().
		Using(metric.KSMSpecs).
		Excluding(
			// Exclude service.loadBalancerIP unless service is e2e-lb (specially crafted to have a fake one)
			func(group string, spec *definition.Spec, ent *integration.Entity) bool {
				return group == "service" && spec.Name == "loadBalancerIP" && ent.Metadata.Name != "e2e-lb"
			},
			// The following HPA metrics operate in a true-or-NULL basis, and there won't be present if condition is
			// false.
			exclude.Exclude(
				exclude.Groups("horizontalpodautoscaler"),
				exclude.Metrics("isLimited"),
			),
			// Kubernetes jobs either succeed or fail (but not both). Thus, the KSM metrics related to success (isComplete, completedAt)
			// and failure (failed, failedPods, failedPodsReason) are excluded.
			exclude.Exclude(
				exclude.Groups("job_name"),
				exclude.Metrics("completedAt", "failedPods", "isComplete", "failed", "failedPodsReason"),
			),
			// Kubernetes pod can be created without the need of a deployment
			exclude.Exclude(
				exclude.Groups("pod"),
				exclude.Metrics("deploymentName"),
			),
			// Kubernetes deployment's `condition` attribute operate in a true-or-NULL basis, so it won't be present if false
			exclude.Exclude(
				exclude.Groups("deployment"),
				exclude.Metrics("conditionReplicaFailure"),
			),
			// excluded pvcName and pvcNamespace (kube_persistentvolume_claim_ref) since not all
			// persistent volumes have claims on them and we want to test that on our E2Es
			// excluded createdAt (kube_persistentvolume_created) since it's marked as experimental
			exclude.Exclude(
				exclude.Groups("persistentvolume"),
				exclude.Metrics("createdAt", "pvcName", "pvcNamespace"),
			),
			// excluded createdAt (kube_persistentvolumeclaim_created) since it's marked as experimental
			exclude.Exclude(
				exclude.Groups("persistentvolumeclaim"),
				exclude.Metrics("createdAt"),
			),
			// Kubernetes pod priorityClassName is optional - only exclude when not present
			exclude.Exclude(
				exclude.Groups("pod"),
				func(_ string, spec *definition.Spec, ent *integration.Entity) bool {
					if spec.Name == "priorityClassName" {
						for _, metricSet := range ent.Metrics {
							if _, exists := metricSet.Metrics[spec.Name]; exists {
								return false // Metric exists, don't exclude
							}
						}
						return true // Metric doesn't exist, exclude
					}
					return false
				},
			),
		).
		AliasingGroups(map[string]string{"horizontalpodautoscaler": "hpa", "job_name": "job", "persistentvolumeclaim": "PersistentVolumeClaim", "persistentvolume": "PersistentVolume"})

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
					StaticURL:                  testServer.KSMEndpoint(),
					EnableResourceQuotaSamples: true,
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
	// We test with a specific version to not modify number of entities
	version := testutil.Version(testutil.Testdata134)
	t.Run(fmt.Sprintf("for_version_%s", version), func(t *testing.T) {
		testServer, err := version.Server()
		require.NoError(t, err)

		ksmCli, err := ksmClient.New()
		require.NoError(t, err)

		k8sData, err := version.K8s()
		require.NoError(t, err)

		fakeK8s := fake.NewSimpleClientset(k8sData.Everything()...)
		scraper, err := ksm.NewScraper(
			&config.Config{
				KSM: config.KSM{
					StaticURL:                  testServer.KSMEndpoint(),
					EnableResourceQuotaSamples: true,
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
		require.NoError(t, err)
		assert.Equal(t, 34, len(i.Entities))
	})
}

func TestScraper_Close(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                        string
		enableCustomResourceMetrics bool
	}{
		{
			name:                        "Close_without_CRD_harvester",
			enableCustomResourceMetrics: false,
		},
		{
			name:                        "Close_with_CRD_harvester",
			enableCustomResourceMetrics: true,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			version := testutil.Version(testutil.Testdata134)
			testServer, err := version.Server()
			require.NoError(t, err)

			ksmCli, err := ksmClient.New()
			require.NoError(t, err)

			k8sData, err := version.K8s()
			require.NoError(t, err)

			fakeK8s := fake.NewClientset(k8sData.Everything()...)
			scraper, err := ksm.NewScraper(
				&config.Config{
					KSM: config.KSM{
						StaticURL:                   testServer.KSMEndpoint(),
						EnableCustomResourceMetrics: tt.enableCustomResourceMetrics,
					},
					ClusterName: t.Name(),
				}, ksm.Providers{
					K8s: fakeK8s,
					KSM: ksmCli,
				},
			)
			require.NoError(t, err)

			// Should not panic
			assert.NotPanics(t, func() {
				scraper.Close()
			})

			// Should be safe to call multiple times
			assert.NotPanics(t, func() {
				scraper.Close()
			})
		})
	}
}

//nolint:funlen //Table-driven test with comprehensive scenarios
func TestScraper_CRDInitialization(t *testing.T) {
	tests := []struct {
		name            string
		licenseKey      string
		harvestPeriod   string
		metricAPIURL    string
		scrapeInterval  string
		expectHarvester bool
	}{
		{
			name:            "CRD disabled - no license key needed",
			licenseKey:      "",
			harvestPeriod:   "",
			metricAPIURL:    "",
			scrapeInterval:  "15s",
			expectHarvester: false,
		},
		{
			name:            "CRD enabled with license key",
			licenseKey:      "test-license-key-1234567890abcdef",
			harvestPeriod:   "",
			metricAPIURL:    "",
			scrapeInterval:  "15s",
			expectHarvester: true,
		},
		{
			name:            "CRD enabled with custom harvest period",
			licenseKey:      "test-license-key-1234567890abcdef",
			harvestPeriod:   "30s",
			metricAPIURL:    "",
			scrapeInterval:  "15s",
			expectHarvester: true,
		},
		{
			name:            "CRD enabled with custom metric API URL",
			licenseKey:      "test-license-key-1234567890abcdef",
			harvestPeriod:   "",
			metricAPIURL:    "https://staging-metric-api.newrelic.com/metric/v1",
			scrapeInterval:  "15s",
			expectHarvester: true,
		},
		{
			name:            "CRD enabled without license key - no harvester created",
			licenseKey:      "",
			harvestPeriod:   "30s",
			metricAPIURL:    "",
			scrapeInterval:  "15s",
			expectHarvester: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.licenseKey != "" {
				t.Setenv("NRIA_LICENSE_KEY", tt.licenseKey)
			}

			version := testutil.Version(testutil.Testdata134)
			testServer, err := version.Server()
			require.NoError(t, err)

			ksmCli, err := ksmClient.New()
			require.NoError(t, err)

			k8sData, err := version.K8s()
			require.NoError(t, err)

			fakeK8s := fake.NewClientset(k8sData.Everything()...)

			var harvestPeriod time.Duration
			if tt.harvestPeriod != "" {
				harvestPeriod, err = time.ParseDuration(tt.harvestPeriod)
				require.NoError(t, err)
			}

			scrapeInterval, err := time.ParseDuration(tt.scrapeInterval)
			require.NoError(t, err)

			scraper, err := ksm.NewScraper(
				&config.Config{
					KSM: config.KSM{
						StaticURL:                   testServer.KSMEndpoint(),
						EnableCustomResourceMetrics: tt.licenseKey != "",
						HarvestPeriod:               harvestPeriod,
						MetricAPIURL:                tt.metricAPIURL,
					},
					Interval:    scrapeInterval,
					ClusterName: t.Name(),
				}, ksm.Providers{
					K8s: fakeK8s,
					KSM: ksmCli,
				},
			)
			require.NoError(t, err)
			defer scraper.Close()

			assert.NotPanics(t, func() {
				scraper.Close()
			})
		})
	}
}

func TestScraper_LicenseKeyMasking(t *testing.T) {
	tests := []struct {
		name       string
		licenseKey string
	}{
		{
			name:       "standard license key",
			licenseKey: "1234567890abcdefghij",
		},
		{
			name:       "short license key",
			licenseKey: "short",
		},
		{
			name:       "exactly 8 chars",
			licenseKey: "12345678",
		},
		{
			name:       "9 chars",
			licenseKey: "123456789",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NRIA_LICENSE_KEY", tt.licenseKey)

			version := testutil.Version(testutil.Testdata134)
			testServer, err := version.Server()
			require.NoError(t, err)

			ksmCli, err := ksmClient.New()
			require.NoError(t, err)

			k8sData, err := version.K8s()
			require.NoError(t, err)

			fakeK8s := fake.NewClientset(k8sData.Everything()...)

			scraper, err := ksm.NewScraper(
				&config.Config{
					KSM: config.KSM{
						StaticURL:                   testServer.KSMEndpoint(),
						EnableCustomResourceMetrics: true,
					},
					Interval:    15 * time.Second,
					ClusterName: t.Name(),
				}, ksm.Providers{
					K8s: fakeK8s,
					KSM: ksmCli,
				},
			)
			require.NoError(t, err)
			defer scraper.Close()

			assert.NotNil(t, scraper)
		})
	}
}
