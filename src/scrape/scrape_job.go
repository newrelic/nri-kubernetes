package scrape

import (
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/version"

	"github.com/newrelic/nri-kubernetes/v2/src/data"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
)

// NewScrapeJob creates a new Scrape Job with the given attributes
func NewScrapeJob(name string, grouper data.Grouper, specs definition.SpecGroups) *Job {
	return &Job{
		Name:    name,
		Grouper: grouper,
		Specs:   specs,
	}
}

// Job hold all information specific to a certain Scrape Job, e.g.: where do I get the data from, and what data
type Job struct {
	Name    string
	Grouper data.Grouper
	Specs   definition.SpecGroups
}

// Populate will get the data using the given Group, transform it, and push it to the given Integration
func (s *Job) Populate(
	integration *sdk.IntegrationProtocol2,
	clusterName string,
	logger *logrus.Logger,
	k8sVersion *version.Info,
) data.PopulateResult {
	groups, errs := s.Grouper.Group(s.Specs)
	if errs != nil && len(errs.Errors) > 0 {
		if !errs.Recoverable {
			return data.PopulateResult{
				Errors:    errs.Errors,
				Populated: false,
			}
		}

		logger.Warnf("%s", errs)
	}

	return metric.NewK8sPopulator().Populate(groups, s.Specs, integration, clusterName, k8sVersion)
}
