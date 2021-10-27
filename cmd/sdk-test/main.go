package main

import (
	"github.com/newrelic/infra-integrations-sdk/integration"
)

type argumentList struct {
	Timeout int `default:"5000" help:"timeout in milliseconds for calling metrics sources"`
}

func main() {
	run(integration.Args(&argumentList{}))
	// panic("err")
}

func run(extraOptions ...integration.Option) {
	_, err := integration.New("integrationName", "integrationVersion", extraOptions...)
	if err != nil {
		panic(err)
	}
}
