package definition

import (
	"fmt"
)

type PopulateErr struct {
	EntityID string
	Err error
}

type FailedFetchMetricErr struct {
	MetricName string
	Err error
}

type FailedComputeMetricValueErr struct {
	//MetricName string
	Err        error
}

func (e PopulateErr) Error() string {
	return fmt.Sprintf("error populating metric for entity ID '%s': %s", e.EntityID, e.Err)
}

func (e FailedFetchMetricErr) Error() string {
	return fmt.Sprintf("cannot fetch value for metric '%s', %s", e.MetricName, e.Err)
}

func (e FailedComputeMetricValueErr) Error() string {
	return fmt.Sprintf("failed to compute metric value: '%s'", e.Err)
}
