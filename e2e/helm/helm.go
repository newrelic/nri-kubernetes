package helm

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v2/e2e/timer"
)

const _helmBinary = "helm"

// InstallRelease installs a chart release
func InstallRelease(releaseName, path, context string, logger *logrus.Logger, config ...string) error {
	defer timer.Track(time.Now(), "Helm InstallRelease", logger)
	args := []string{
		"install",
		releaseName,
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
		return fmt.Errorf("%w - %s", err, string(o))
	}
	return nil
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
		return fmt.Errorf("%w - %s", err, string(o))
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
		return fmt.Errorf("%w - %s", err, string(o))
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
		return fmt.Errorf("%w - %s", err, string(o))
	}

	return nil
}
