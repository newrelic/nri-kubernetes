package network

// DefaultInterface returns the default interface named used by the OS.
func DefaultInterface(routeFile string) (string, error) {
	return getDefaultInterface(routeFile)
}
