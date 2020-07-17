package jsonschema

import (
	"fmt"

	"path/filepath"

	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/newrelic/nri-kubernetes/e2e/jsonschema/schema"
	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
)

// EventTypeToSchemaFilename maps event types with their json schema.
type EventTypeToSchemaFilename map[string]string

// ErrMatch is the error that Match function returns
type ErrMatch struct {
	errs []error
}

func (errMatch ErrMatch) Error() string {
	var out string
	for _, e := range errMatch.errs {
		out = fmt.Sprintf("\n%s\t- %s\n", out, e)
	}

	return out
}

// MatchIntegration matches an integration against a JSON schema defined for an
// Infrastructure Integration.
func MatchIntegration(o *sdk.IntegrationProtocol2) error {
	return validate(gojsonschema.NewStringLoader(schema.IntegrationSchema), gojsonschema.NewGoLoader(o))
}

// MatchEntities matches metric sets of entities against a set of JSON schema
// for each event type.
func MatchEntities(d []*sdk.EntityData, m EventTypeToSchemaFilename, schemasDir string) error {
	var errs []error
	missingSchemas := make(map[string]struct{})
	foundTypes := make(map[string]struct{})
	for _, e := range d {
		for _, ms := range e.Metrics {
			if t, ok := m[ms["event_type"].(string)]; ok {
				foundTypes[ms["event_type"].(string)] = struct{}{}
				fp, err := schemaFilepath(t, schemasDir)
				if err != nil {
					errs = append(errs, fmt.Errorf("%s schema not found", t))
					continue
				}

				err = validate(gojsonschema.NewReferenceLoader(fp), gojsonschema.NewGoLoader(ms))
				if err != nil {
					errs = append(errs, fmt.Errorf("%s:%s %s:\n%s", e.Entity.Type, e.Entity.Name, ms["event_type"], err))
				}
			} else {
				missingSchemas[ms["event_type"].(string)] = struct{}{}
			}
		}
	}

	var terr string
	for t := range m {
		if _, ok := foundTypes[t]; !ok {
			terr = fmt.Sprintf("%s%s, ", terr, t)
		}
	}
	if len(terr) > 0 {
		errs = append(errs, fmt.Errorf("mandatory types were not found: %s", terr))
	}

	if len(missingSchemas) > 0 {
		e := fmt.Sprint("some types were not validated because no schema was found: ")
		for t := range missingSchemas {
			e = fmt.Sprintf("%s%s, ", e, t)
		}

		errs = append(errs, errors.New(e))
	}

	if len(errs) > 0 {
		return ErrMatch{errs: errs}
	}

	return nil
}

func schemaFilepath(filename string, dir string) (string, error) {
	schemas := filepath.Join(dir, filename)
	abs, err := filepath.Abs(schemas)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("file://%s", abs), nil
}

func validate(schemaLoader, documentLoader gojsonschema.JSONLoader) error {
	r, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return err
	}

	if r.Valid() {
		return nil
	}

	var validationErrsMsg string
	for _, desc := range r.Errors() {
		validationErrsMsg = fmt.Sprintf("%s\t\t- %s\n", validationErrsMsg, desc)
	}

	return errors.New(validationErrsMsg)
}
