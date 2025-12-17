//go:build windows

package network

import (
	"fmt"
	"os/exec"
	"strings"
)

// getDefaultInterface returns the default network interface on Windows
// by querying the default route using PowerShell's Get-NetRoute.
func getDefaultInterface(_ string) (string, error) {
	// PowerShell command to get the interface alias for the default route (0.0.0.0/0)
	// Get-NetRoute -DestinationPrefix "0.0.0.0/0" | Select-Object -First 1 -ExpandProperty InterfaceAlias
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty InterfaceAlias")

	output, err := cmd.Output()
	if err != nil {
		// If PowerShell command fails, fall back to common Windows interface name
		return "Ethernet", nil
	}

	iface := strings.TrimSpace(string(output))
	if iface == "" {
		// No default route found, use common name
		return "Ethernet", nil
	}

	return iface, nil
}

// FindDefaultInterfaceForTest is exported for testing purposes
func FindDefaultInterfaceForTest() (string, error) {
	return getDefaultInterface("")
}

// GetDefaultInterfaceDirect is a helper for testing without file paths
func GetDefaultInterfaceDirect() (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty InterfaceAlias")

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get default interface: %w", err)
	}

	iface := strings.TrimSpace(string(output))
	if iface == "" {
		return "", fmt.Errorf("no default route found")
	}

	return iface, nil
}
