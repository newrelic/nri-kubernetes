package main

import (
	"os"

	"github.com/newrelic/infra-integrations-sdk/log"
)

func main() {
	if err := run(); err != nil {
		log.Error("Error %v", err)
		os.Exit(1)
	}
}
