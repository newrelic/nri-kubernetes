package network

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultRouteFile   = "/proc/net/route"
	sep                = "\t" // routes file field separator
	destinationField   = 1    // routes file field containing hex destination address
	interfaceNameField = 0    // routes file field containing interface name
)

// allowedRouteFilePrefixes defines the directory prefixes that NetworkRouteFile is allowed to reference.
var allowedRouteFilePrefixes = []string{"/proc/", "/sys/", "/host/proc/", "/host/sys/"}

func getDefaultInterface(routeFile string) (string, error) {
	if routeFile == "" {
		routeFile = defaultRouteFile
	}

	if err := validateRouteFilePath(routeFile); err != nil {
		return "", err
	}

	routes, err := routeFileContent(routeFile)
	if err != nil {
		return "", fmt.Errorf("getting routes content from file %s: %w", routeFile, err)
	}
	return findDefaultInterface(routes)
}

// validateRouteFilePath checks that the route file path is under an allowed prefix
// to prevent path traversal via user-controllable config.
func validateRouteFilePath(path string) error {
	cleaned := filepath.Clean(path)
	for _, prefix := range allowedRouteFilePrefixes {
		if strings.HasPrefix(cleaned, prefix) {
			return nil
		}
	}
	return fmt.Errorf("networkRouteFile %q is not under an allowed path (%v)", path, allowedRouteFilePrefixes)
}

func routeFileContent(routeFile string) ([]byte, error) {
	f, err := os.Open(routeFile)
	if err != nil {
		return nil, fmt.Errorf("Can't access %s", routeFile)
	}
	defer func() {
		_ = f.Close()
	}()

	return io.ReadAll(f)
}

// findDefaultInterface parses the route file and returns the name
// of the default interface, that is, the interface with Destination = 0
func findDefaultInterface(route []byte) (string, error) {
	/* /proc/net/route file:
	   Iface   Destination Gateway     Flags   RefCnt  Use Metric  Mask
	   eno1    00000000    C900A8C0    0003    0   0   100 00000000    0   00
	   eno1    0000A8C0    00000000    0001    0   0   100 00FFFFFF    0   00
	*/
	scanner := bufio.NewScanner(bytes.NewReader(route))

	// Skip header line
	if !scanner.Scan() {
		return "", fmt.Errorf("invalid linux route file: %s", route)
	}

	for scanner.Scan() {
		row := scanner.Text()
		tokens := strings.Split(row, sep)
		if len(tokens) <= destinationField {
			return "", fmt.Errorf("invalid row '%s' in route file", row)
		}

		destinationHex := "0x" + tokens[destinationField]

		// Cast hex address to int
		destination, err := strconv.ParseInt(destinationHex, 0, 64)
		if err != nil {
			return "", fmt.Errorf("parsing destination field hex '%s' in row '%s': %w", destinationHex, row, err)
		}

		// The default interface is the one that's 0
		if destination == 0 {
			return tokens[interfaceNameField], nil
		}
	}
	return "", errors.New("couldn't find interface with default destination")
}
