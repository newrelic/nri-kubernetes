package main

import (
	"os"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/log"
)

var logger log.Logger

func main() {
	args := &argumentList{}
	sdkArgs.SetupArgs(args)

	logger = log.NewStdErr(args.Verbose)

	if err := run(args); err != nil {
		log.Error("Error %v", err)
		os.Exit(1)
	}
}
