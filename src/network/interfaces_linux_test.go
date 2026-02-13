package network

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultInterface(t *testing.T) {
	t.Parallel()

	f, err := filepath.Abs("./testdata/route")
	require.NoError(t, err)

	// Read the file content directly to test parsing logic
	// without path validation (test file is not under /proc/ or /sys/).
	content, err := routeFileContent(f)
	require.NoError(t, err)
	i, err := findDefaultInterface(content)
	require.NoError(t, err)
	assert.Equal(t, "ens5", i)
}

func TestValidateRouteFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "valid /proc path", path: "/proc/net/route", wantErr: false},
		{name: "valid /sys path", path: "/sys/class/net/route", wantErr: false},
		{name: "valid /host/proc path", path: "/host/proc/1/net/route", wantErr: false},
		{name: "valid /host/sys path", path: "/host/sys/class/net/route", wantErr: false},
		{name: "traversal out of /proc", path: "/proc/../etc/passwd", wantErr: true},
		{name: "arbitrary path", path: "/etc/passwd", wantErr: true},
		{name: "relative path", path: "../../etc/passwd", wantErr: true},
		{name: "empty path uses default", path: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateRouteFilePath(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultInterfaceRejectsUnsafePath(t *testing.T) {
	t.Parallel()

	_, err := DefaultInterface("/etc/passwd")
	assert.True(t, errors.Is(err, errRouteFilePathNotAllowed))
}
