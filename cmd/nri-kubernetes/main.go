package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/newrelic/infra-integrations-sdk/integration"

	"github.com/newrelic/nri-kubernetes/v2/cli"
)

func main() {
	// NewRelic SDK use ExitOnError, which makes it impossible to handle errors properly.
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error)
	args := &cli.ArgumentList{}

	go func() {
		errCh <- cli.Run(ctx, cli.RunOptions{
			ArgumentList: args,
			ExtraOptions: []integration.Option{
				integration.Args(args),
			},
		})
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigs:
		cancel()
	case err := <-errCh:
		if err != nil {
			fmt.Printf("Running integration failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}
