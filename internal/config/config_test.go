package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
)

const fakeDataDir = "testdata"
const workingData = "config"

// Added namespaceSelector config in a separate yaml, this way we can be sure there is no error in its absence.
const workingDataWithNamespaceFilters = "config_with_namespace_filter"
const wrongDataWithNamespaceFilterMatchLabels = "config_with_namespace_filter_wrong_match_labels"
const wrongDataWithNamespaceFiltersMatchExpressions = "config_with_namespace_filter_wrong_match_expressions"
const unexpectedFields = "config_with_unexpected_fields"
const configWithNewDefaults = "config_with_new_defaults"

func TestLoadConfig(t *testing.T) {

	t.Run("succeeds", func(t *testing.T) {
		t.Parallel()

		c, err := config.LoadConfig(fakeDataDir, workingData)
		require.NoError(t, err)
		require.Equal(t, "dummy_cluster", c.ClusterName)

		t.Run("with_env_precedence", func(t *testing.T) {
			_ = os.Setenv("NRI_KUBERNETES_CLUSTERNAME", "different_value")
			_ = os.Setenv("NRI_KUBERNETES_NODENAME", "fake-node")

			c, err := config.LoadConfig(fakeDataDir, workingData)
			require.NoError(t, err)
			require.Equal(t, "different_value", c.ClusterName)
			require.Equal(t, "fake-node", c.NodeName)
		})

		t.Run("takes_test_connection_endpoint_from_env", func(t *testing.T) {
			t.Parallel()
			_ = os.Setenv("NRI_KUBERNETES_TESTCONNECTIONENDPOINT", "metrics")

			c, err := config.LoadConfig(fakeDataDir, workingData)
			require.NoError(t, err)
			require.Equal(t, "metrics", c.TestConnectionEndpoint)
		})

		t.Run("takes_fetch_pod_from_kube_service_from_env", func(t *testing.T) {
			t.Parallel()
			_ = os.Setenv("NRI_KUBERNETES_KUBELET_FETCHPODSFROMKUBESERVICE", "true")

			c, err := config.LoadConfig(fakeDataDir, workingData)
			require.NoError(t, err)
			require.Equal(t, true, c.FetchPodsFromKubeService)
		})
	})
	// This test checks that viper custom key delimiter is working as expected by using the old default dot delimiter
	// as key.
	t.Run("succeeds_when_dot_character_in_key", func(t *testing.T) {
		t.Parallel()

		c, err := config.LoadConfig(fakeDataDir, workingDataWithNamespaceFilters)
		require.NoError(t, err)
		require.Contains(t, c.NamespaceSelector.MatchLabels, "newrelic.com/scrape")
		require.Equal(t, "newrelic.com/scrape", c.NamespaceSelector.MatchExpressions[0].Key)
	})
	t.Run("fail_when_bad_namespace_filter_match_labels_values", func(t *testing.T) {
		t.Parallel()

		_, err := config.LoadConfig(fakeDataDir, wrongDataWithNamespaceFilterMatchLabels)
		require.ErrorIs(t, err, config.ErrInvalidMatchLabelsValue)
	})
	t.Run("fail_when_bad_namespace_filter_match_expressions_values", func(t *testing.T) {
		t.Parallel()

		_, err := config.LoadConfig(fakeDataDir, wrongDataWithNamespaceFiltersMatchExpressions)
		require.ErrorIs(t, err, config.ErrInvalidMatchExpressionsValue)
	})
	t.Run("fail_due_to_unexpected_data", func(t *testing.T) {
		t.Parallel()

		_, err := config.LoadConfig(fakeDataDir, unexpectedFields)
		require.Error(t, err)
	})
	t.Run("fail_due_to_missing_file", func(t *testing.T) {
		t.Parallel()

		_, err := config.LoadConfig(fakeDataDir, "not-existing-file")
		require.Error(t, err)
	})
}

func TestEnableResourceQuotaSamples(t *testing.T) {
	const envKey = "NRI_KUBERNETES_KSM_ENABLERESOURCEQUOTASAMPLES"
	originalValue, wasSet := os.LookupEnv(envKey)
	defer func() {
		if wasSet {
			os.Setenv(envKey, originalValue)
		} else {
			os.Unsetenv(envKey)
		}
	}()

	// Set the desired value for this specific test.
	os.Setenv(envKey, "true")

	// Run the test logic.
	cfg, err := config.LoadConfig(fakeDataDir, workingData)
	require.NoError(t, err)
	require.True(t, cfg.EnableResourceQuotaSamples)
}

func TestKubeletInitRetryFieldsOptional(t *testing.T) {
	t.Parallel()

	t.Run("defaults_to_0s_when_missing", func(t *testing.T) {
		t.Parallel()

		// Load config without initTimeout and initBackoff fields (backward compatibility test)
		cfg, err := config.LoadConfig(fakeDataDir, workingData)
		require.NoError(t, err)

		// When fields are missing, they should default to 0s (legacy behavior: no retry)
		require.Equal(t, 0, int(cfg.InitTimeout.Seconds()),
			"initTimeout should be 0s when missing from config (legacy behavior)")
		require.Equal(t, 0, int(cfg.InitBackoff.Seconds()),
			"initBackoff should be 0s when missing from config (legacy behavior)")
	})

	t.Run("uses_configured_values_when_present", func(t *testing.T) {
		t.Parallel()

		// Load config with initTimeout and initBackoff fields present
		cfg, err := config.LoadConfig(fakeDataDir, configWithNewDefaults)
		require.NoError(t, err)

		// When fields are present, they should use the configured values
		require.Equal(t, 180, int(cfg.InitTimeout.Seconds()),
			"initTimeout should be 180s when explicitly set in config")
		require.Equal(t, 5, int(cfg.InitBackoff.Seconds()),
			"initBackoff should be 5s when explicitly set in config")
	})
}
