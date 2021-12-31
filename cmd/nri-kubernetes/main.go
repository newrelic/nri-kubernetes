package main

import (
	"time"

	integration "github.com/newrelic/nri-kubernetes/v2/src"
	"github.com/sirupsen/logrus"
)

const (
	integrationName = "com.newrelic.kubernetes"
)

var (
	integrationVersion = "dev"
	gitCommit          = "unknown"
	buildDate          = time.Now().String()
)

func main() {
	i := integration.New(integration.Metadata{
		Name:      integrationName,
		Version:   integrationVersion,
		GitCommit: gitCommit,
		BuildDate: buildDate,
	})

	i.Logger = logrus.StandardLogger()

	if err := i.Run(); err != nil {
		logrus.Fatalf("Error %v", err)
	}
}
