package scrape

import (
	"errors"
	"fmt"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/integration"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/version"

	"github.com/newrelic/nri-kubernetes/v3/src/data"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
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
		MsTypeGuesser: k8sMetricSetTypeGuesser,
		Groups:        groups,
	}
	ok, populateErrs := definition.IntegrationPopulator(config)

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

// k8sMetricSetTypeGuesser is the metric set type guesser for k8s integrations.
func k8sMetricSetTypeGuesser(_, groupLabel, _ string, _ definition.RawGroups) (string, error) {
	var sampleName string
	for _, s := range strings.Split(groupLabel, "-") {
		sampleName += strings.Title(s)
	}
	return fmt.Sprintf("K8s%vSample", sampleName), nil
}
