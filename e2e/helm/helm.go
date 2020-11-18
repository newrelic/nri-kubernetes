package helm

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/newrelic/nri-kubernetes/e2e/timer"
	"github.com/sirupsen/logrus"
)

// _helmBinary is the path to the helm 2 binary
// Helm v2 is used, and with the release of helm v3 the helm command no longer links to v2.
// Set the path to helm v2 here, for example:
//const _helmBinary = "/usr/local/bin/helm2"
const _helmBinary = "linux-amd64/helm"

// InstallRelease installs a chart release
func InstallRelease(path, context string, logger *logrus.Logger, config ...string) ([]byte, error) {
	defer timer.Track(time.Now(), "Helm InstallRelease", logger)
	args := []string{
		"install",
		path,
		"--wait",
	}

	if len(config) > 0 {
		args = append(args, "--set", strings.Join(config, ","))
	}

	if context != "" {
		args = append(args, "--kube-context", context)
	}

	c := exec.Command(_helmBinary, args...)
	o, err := c.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s - %s", err, string(o))
	}

	return o, nil
}

// DeleteRelease deletes a chart release
func DeleteRelease(release, context string, logger *logrus.Logger) error {
	defer timer.Track(time.Now(), fmt.Sprintf("Helm DeleteRelease: %s", release), logger)
	args := []string{
		"delete",
		release,
	}

	if context != "" {
		args = append(args, "--kube-context", context)
	}

	c := exec.Command(_helmBinary, args...)
	o, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s - %s", err, string(o))
	}

	return nil
}

// DeleteAllReleases deletes all chart releases
func DeleteAllReleases(context string, logger *logrus.Logger) error {
	defer timer.Track(time.Now(), "Helm DeleteAllReleases", logger)
	args := []string{
		"list",
		"--short",
	}

	if context != "" {
		args = append(args, "--kube-context", context)
	}

	c := exec.Command(_helmBinary, args...)
	o, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s - %s", err, string(o))
	}

	scanner := bufio.NewScanner(bytes.NewReader(o))
	for scanner.Scan() {
		err := DeleteRelease(scanner.Text(), context, logger)
		if err != nil {
			return err
		}
	}

	return scanner.Err()
}

// IsRunningHelm3 is a small function to check whether Helm3 is used
func IsRunningHelm3(logger *logrus.Logger) bool {
	c := exec.Command(_helmBinary, "version", "--client", "--short")

	o, err := c.CombinedOutput()
	if err != nil {
		logrus.Infof("Could not determine whether Helm3 is running: %v. Output: %s", err, string(o))
		return false // not sure if Helm3
	}

	return strings.HasPrefix(string(o), "v3.")
}

// Init installs Tiller (the Helm server-side component) onto your cluster
func Init(context string, logger *logrus.Logger, arg ...string) error {
	defer timer.Track(time.Now(), "Helm Init", logger)
	args := append([]string{
		"init",
		"--wait",
	}, arg...)

	if context != "" {
		args = append(args, "--kube-context", context)
	}

	c := exec.Command(_helmBinary, args...)
	o, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s - %s", err, string(o))
	}

	return nil
}

// DependencyBuild builds the dependencies for the e2e chart
func DependencyBuild(context, chart string, logger *logrus.Logger) error {
	defer timer.Track(time.Now(), "Helm DependencyBuild", logger)
	args := []string{
		"dependency",
		"build",
		chart,
	}

	if context != "" {
		args = append(args, "--kube-context", context)
	}

	c := exec.Command(_helmBinary, args...)
	o, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s - %s", err, string(o))
	}

	return nil
}
