package jsonschema

import (
	"encoding/json"
	"testing"

	"os"

	"io/ioutil"

	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/stretchr/testify/assert"
)

var s = map[string]EventTypeToSchemaFilename{
	"dummy-task-name": {
		"TestNodeSample":    "schema-testnode.json",
		"TestServiceSample": "schema-testservice.json",
	},
}

func TestNoError(t *testing.T) {
	c := readTestInput(t, "testdata/input-complete.json")
	i := sdk.IntegrationProtocol2{}
	err := json.Unmarshal(c, &i)
	if err != nil {
		t.Fatal(err)
	}

	err = MatchIntegration(&i)
	assert.NoError(t, err)
	err = MatchEntities(i.Data, s, "testdata")
	assert.NoError(t, err)
}

func TestErrorValidatingInputWithNoData(t *testing.T) {
	c := readTestInput(t, "testdata/input-invalid-nodata.json")
	i := sdk.IntegrationProtocol2{}
	err := json.Unmarshal(c, &i)
	if err != nil {
		t.Fatal(err)
	}

	err = MatchIntegration(&i)
	assert.Contains(t, err.Error(), "data: Array must have at least 1 items")
}

func TestErrorValidatingEventTypes(t *testing.T) {
	c := readTestInput(t, "testdata/input-missing-event-type.json")
	i := sdk.IntegrationProtocol2{}
	err := json.Unmarshal(c, &i)
	if err != nil {
		t.Fatal(err)
	}

	jobMetrics := map[string]EventTypeToSchemaFilename{
		"dummy-job-name": {
			"TestNodeSample":    "testdata/schema-testnode.json",
			"TestServiceSample": "testdata/schema-testservice.json",
			"TestPodSample":     "testdata/schema-testpod.json", // this file doesn't exist, I just want to test with 2 missing types
		},
	}
	err = MatchEntities(i.Data, jobMetrics, "testdata")

	assert.Contains(t, err.Error(), "mandatory types were not found: ")
	assert.Contains(t, err.Error(), "TestServiceSample, ")
	assert.Contains(t, err.Error(), "TestPodSample, ")
}

func TestErrorValidatingTestNode(t *testing.T) {
	c := readTestInput(t, "testdata/input-invalid-testnode.json")
	i := sdk.IntegrationProtocol2{}
	err := json.Unmarshal(c, &i)
	if err != nil {
		t.Fatal(err)
	}

	err = MatchEntities(i.Data, s, "testdata")
	assert.Contains(t, err.Error(), "test-node:node1-dsn.compute.internal TestNodeSample")
	assert.Contains(t, err.Error(), "capacity: capacity is required")
	assert.Contains(t, err.Error(), "test-node:node2-dsn.compute.internal TestNodeSample")
	assert.Contains(t, err.Error(), "cpuUsedCores: Invalid type. Expected: number, given: string")
}

func readTestInput(t *testing.T, filepath string) []byte {
	f, err := os.Open(filepath)
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	c, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}

	return c
}
