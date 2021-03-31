/*
Package featureflag holds the feature flags for the integration.
Feature flags are conditionals that tell us if a feature should be
enable or disable.

To keep it simple, since we only have one feature flag right now, it just
a single function. In case we add more feature flags we can refactor it
into a more robust and flexible implementation.
*/
package featureflag

import (
	"regexp"
	"strconv"

	"k8s.io/apimachinery/pkg/version"
)

// StaticPodsStatus checks that the kubernetes server version
// is greater that 1.14 which is when the kubelete started to sync the status
// for static pods.
// https://github.com/kubernetes/kubernetes/pull/77661
func StaticPodsStatus(v *version.Info) bool {
	if v == nil {
		return false
	}
	// this regex is used to strip any symbol from the version and take the first left match after
	r := regexp.MustCompile("([0-9]+)")
	major, err := strconv.Atoi(r.FindString(v.Major))
	if err != nil {
		return false
	}
	if major > 1 {
		return true
	}
	minor, err := strconv.Atoi(r.FindString(v.Minor))
	if err != nil {
		return false
	}
	return minor > 14
}
