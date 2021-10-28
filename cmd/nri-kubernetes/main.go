package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
)

type argumentList struct {
	IntervalSeconds int `default:"30" help:"interval in seconds at which integration data should be published"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error)

	args := &argumentList{}

	go func() {
		errCh <- run(ctx, runOptions{
			argumentList: args,
			extraOptions: []integration.Option{
				integration.Args(args),
			},
		})
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for {
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
}

type runOptions struct {
	*argumentList
	extraOptions []integration.Option
}

func run(ctx context.Context, options runOptions) error {
	i, err := integration.New("integrationName", "integrationVersion", options.extraOptions...)
	if err != nil {
		return fmt.Errorf("initializing integration: %w", err)
	}

	if err := feedAndPublish(i); err != nil {
		return fmt.Errorf("feeding integration: %w", err)
	}

	ticker := time.NewTicker(time.Duration(int64(options.IntervalSeconds) * int64(time.Second)))

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := feedAndPublish(i); err != nil {
				return fmt.Errorf("feeding integration: %w", err)
			}
		}
	}
}

func feedAndPublish(i *integration.Integration) error {
	e, err := i.Entity("entityName", "entityNamespace")
	if err != nil {
		return fmt.Errorf("creating entity: %w", err)
	}

	ms := e.NewMetricSet("testMetricSet")

	if err := ms.SetMetric("clusterK8sVersion", "test", metric.ATTRIBUTE); err != nil {
		return fmt.Errorf("setting metric: %w", err)
	}

	if err := e.Inventory.SetItem("exampleItem", "exampleValue", metric.ATTRIBUTE); err != nil {
		return fmt.Errorf("setting inventory item: %w", err)
	}

	i.Publish()

	return nil
}
