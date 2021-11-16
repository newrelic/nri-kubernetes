package testutil

import (
	"embed"
)

//go:embed data
var testDataDir embed.FS

// Name of the root folder in embed.FS
const testDataRootDir = "data"

// Version represents a kubernetes version. Mock servers can be instantiated to return known output for a given version.
type Version string

// Server returns an HTTP Server for the given version, ready to serve static endpoints for KSM, Kubelet and CP components.
func (v Version) Server() (*Server, error) {
	return newServer(v)
}

// List of all the versions we have testdata for.
// When adding a new version:
// - REMEMBER TO ADD IT TO AllVersions() BELOW.
// - UPDATE LatestVersion() BELOW IF NEEDED

const (
	Testdata116 = "1_16"
	Testdata118 = "1_18"
)

// LatestVersion returns the latest version we have test data for.
func LatestVersion() Version {
	return Testdata118
}

// AllVersions returns a list of versions we have test data for.
func AllVersions() []Version {
	return []Version{
		Testdata116,
		Testdata118,
	}
}
