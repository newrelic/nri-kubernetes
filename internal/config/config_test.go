package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
)

const fakeDataDir = "testdata"
const workingData = "config"
const unexpectedFields = "config_with_unexpected_fields"

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
