package testutil

import (
	"embed"
	"sort"
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

// K8s returns a helper that provide fake instances of K8s objects, ready to use with the kubernetes fake client.
func (v Version) K8s() (K8s, error) {
	return newK8s(v)
}

// List of all the versions we have testdata for.
// When adding a new version:
// - REMEMBER TO ADD IT TO AllVersions() BELOW.

const (
	Testdata130 = "1_30"
	Testdata131 = "1_31"
	Testdata132 = "1_32"
	Testdata133 = "1_33"
	Testdata134 = "1_34"
)

// AllVersions returns a list of versions we have test data for.
// PLEASE ADD NEW VERSIONS HERE AS WELL.
// PLEASE KEEP THIS LIST SORTED, WITH NEWER RELEASES LAST IN THE LIST.
func AllVersions() []Version {
	return []Version{
		Testdata130,
		Testdata131,
		Testdata132,
		Testdata133,
		Testdata134,
	}
}

// LatestVersion returns the latest version we have test data for.
func LatestVersion() Version {
	allVersions := AllVersions()
	return allVersions[len(allVersions)-1]
}

// IsBelow returns true when a is below b in the versions we have test data for. It assumes test data for
// both versions exists and the corresponding `AllVersions()` list is properly sorted.
func IsBelow(a, b Version) bool {
	allVersions := AllVersions()
	aIndex := sort.Search(len(allVersions), func(i int) bool { return allVersions[i] >= a })
	bIndex := sort.Search(len(allVersions), func(i int) bool { return allVersions[i] >= b })
	return aIndex < bIndex
}
