package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
)

type ArgumentList struct {
	IntervalSeconds int `default:"30" help:"interval in seconds at which integration data should be published"`
}

type RunOptions struct {
	*ArgumentList
	ExtraOptions []integration.Option
}

func Run(ctx context.Context, options RunOptions) error {
	i, err := integration.New("integrationName", "integrationVersion", options.ExtraOptions...)
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

	if err := i.Publish(); err != nil {
		return fmt.Errorf("publishing metrics: %w", err)
	}

	return nil
}
