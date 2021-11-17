package network

type defaultInterfaceFunc func(string) (string, error)

// DefaultInterface returns the default interface named used by the OS.
func DefaultInterface(routeFile string) (string, error) {
	return getDefaultInterface(routeFile)
}
