package scrape

import (
	"errors"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v3/src/populator"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/version"

	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/src/data"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

// JobOpt are options that can be used to configure the ScrapeJob
type JobOpt func(s *Job)

// NewScrapeJob creates a new Scrape Job with the given attributes
func NewScrapeJob(name string, grouper data.Grouper, specs definition.SpecGroups, options ...JobOpt) *Job {
	job := &Job{
		Name:    name,
		Grouper: grouper,
		Specs:   specs,
	}

	for _, opt := range options {
		opt(job)
	}

	return job
}

// Job hold all information specific to a certain Scrape Job, e.g.: where do I get the data from, and what data
type Job struct {
	Name     string
	Grouper  data.Grouper
	Specs    definition.SpecGroups
	Filterer discovery.NamespaceFilterer
}

// JobWithFilterer returns an OptionFunc to add a Filterer.
func JobWithFilterer(filterer discovery.NamespaceFilterer) JobOpt {
	return func(j *Job) {
		j.Filterer = filterer
	}
}

// Populate will get the data using the given Group, transform it, and push it to the given Integration
func (s *Job) Populate(
	i *integration.Integration,
	clusterName string,
	logger *log.Logger,
	k8sVersion *version.Info,
) data.PopulateResult {
	groups, errs := s.Grouper.Group(s.Specs)
	if errs != nil {
		if !errs.Recoverable {
			return data.PopulateResult{
				Errors: errs.Errors,
			}
		}

		logger.Tracef("%s", errs)
	}

	config := &definition.IntegrationPopulateConfig{
		Integration:   i,
		ClusterName:   clusterName,
		K8sVersion:    k8sVersion,
		Specs:         s.Specs,
		MsTypeGuesser: definition.K8sMetricSetTypeGuesser,
		Groups:        groups,
		Filterer:      s.Filterer,
	}
	ok, populateErrs := populator.IntegrationPopulator(config)

	if len(populateErrs) > 0 {
		return data.PopulateResult{Errors: populateErrs, Populated: ok}
	}

	// This should not happen ideally if no errors were reported.
	if !ok {
		return data.PopulateResult{
			Errors: []error{errors.New("no data was populated")},
		}
	}

	return data.PopulateResult{Populated: true}
}
