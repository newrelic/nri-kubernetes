//go:build windows

package network

import (
	"testing"
)

func TestGetDefaultInterface(t *testing.T) {
	// This test can only run on Windows and requires network connectivity
	iface, err := getDefaultInterface("")
	if err != nil {
		t.Logf("Warning: Could not get default interface: %v (this is expected if no network is available)", err)
		return
	}

	if iface == "" {
		t.Error("Expected non-empty interface name")
	}

	t.Logf("Default interface: %s", iface)
}

func TestGetDefaultInterfaceDirect(t *testing.T) {
	// This test can only run on Windows and requires network connectivity
	iface, err := GetDefaultInterfaceDirect()
	if err != nil {
		t.Logf("Warning: Could not get default interface: %v (this is expected if no network is available)", err)
		return
	}

	if iface == "" {
		t.Error("Expected non-empty interface name")
	}

	t.Logf("Default interface: %s", iface)
}
