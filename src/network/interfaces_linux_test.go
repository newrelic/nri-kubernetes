package network

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultInterface(t *testing.T) {
	f, err := filepath.Abs("./testdata/route")
	require.NoError(t, err)
	i, err := DefaultInterface(f)
	require.NoError(t, err)
	assert.Equal(t, "ens5", i)
}
