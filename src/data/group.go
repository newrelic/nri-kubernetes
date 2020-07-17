package data

import (
	"fmt"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/newrelic/nri-kubernetes/src/definition"
)

// Grouper groups raw data by any desired label such object (pod, container...).
type Grouper interface {
	Group(definition.SpecGroups) (definition.RawGroups, *ErrorGroup)
}

// Populator populates a given integration with grouped raw data.
type Populator interface {
	Populate(definition.RawGroups, definition.SpecGroups, *sdk.IntegrationProtocol2, string) *PopulateErr
}

// ErrorGroup groups errors that can be recoverable (the execution can continue) or not
type ErrorGroup struct {
	Recoverable bool
	Errors      []error
}

// Append appends the errors passed as argument to the errors slice of the receiver object.
func (g *ErrorGroup) Append(errs ...error) {
	g.Errors = append(g.Errors, errs...)
}

// String shows a comma-separated string representation of all the error messages
func (g ErrorGroup) String() string {
	strs := make([]string, 0, len(g.Errors))
	for _, err := range g.Errors {
		strs = append(strs, err.Error())
	}
	var recoverable string
	if g.Recoverable {
		recoverable = "Recoverable"
	} else {
		recoverable = "Non-recoverable"
	}
	return fmt.Sprintf("%s error group: %s", recoverable, strings.Join(strs, ", "))
}

// Error implements error interface.
func (g ErrorGroup) Error() string {
	return g.String()
}

// PopulateErr represents an error or a bunch of errors during populate.
type PopulateErr struct {
	Errs      []error
	Populated bool
}

// Error implements error interface
func (e PopulateErr) Error() string {
	s := "populate errors:"
	for _, err := range e.Errs {
		s = fmt.Sprintf("%s\n%s", s, err)
	}

	return s
}
