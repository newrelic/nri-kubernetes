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

	args := &cli.ArgumentList{}

	if err := runWithSignalHandling(func(ctx context.Context) error {
		return cli.Run(ctx, cli.RunOptions{
			ArgumentList: args,
			ExtraOptions: []integration.Option{
				integration.Args(args),
			},
		})
	}); err != nil {
		fmt.Printf("Running integration failed: %v\n", err)
		os.Exit(1)
	}
}

func runWithSignalHandling(run func(ctx context.Context) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error)

	go func() {
		errCh <- run(ctx)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigs:
			cancel()
		case err := <-errCh:
			return err
		}
	}
}
