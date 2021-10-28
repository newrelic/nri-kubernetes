package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/newrelic/infra-integrations-sdk/integration"
)

type argumentList struct {
	Timeout int `default:"5000" help:"timeout in milliseconds for calling metrics sources"`
}

func main() {
	run(integration.Args(&argumentList{}))

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
}

func run(extraOptions ...integration.Option) {
	_, err := integration.New("integrationName", "integrationVersion", extraOptions...)
	if err != nil {
		panic(err)
	}
}
