//go:build !linux

package network

func getDefaultInterface(_ string) (string, error) {
	return "eth0", nil
}
