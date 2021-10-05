package jsonschema

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var s = map[string]EventTypeToSchemaFilename{
	"dummy-task-name": {
		"TestNodeSample":    "schema-testnode.json",
		"TestServiceSample": "schema-testservice.json",
	},
}

func TestNoError(t *testing.T) {
	c := readTestInput(t, "testdata/input-complete.json")
	i, err := integration.New("e2e", "0.0.0")
	require.NoError(t, err)

	err = json.Unmarshal(c, i)
	require.NoError(t, err)

	err = MatchIntegration(i)
	assert.NoError(t, err)
	err = MatchEntities(i.Entities, s, "testdata")
	assert.NoError(t, err)
}

func TestErrorValidatingInputWithNoData(t *testing.T) {
	c := readTestInput(t, "testdata/input-invalid-nodata.json")
	i, err := integration.New("e2e", "0.0.0")
	require.NoError(t, err)

	err = json.Unmarshal(c, i)
	require.NoError(t, err)

	err = MatchIntegration(i)
	assert.Contains(t, err.Error(), "data: Array must have at least 1 items")
}

func TestErrorValidatingEventTypes(t *testing.T) {
	c := readTestInput(t, "testdata/input-missing-event-type.json")
	i, err := integration.New("e2e", "0.0.0")
	require.NoError(t, err)

	err = json.Unmarshal(c, i)
	require.NoError(t, err)

	jobMetrics := map[string]EventTypeToSchemaFilename{
		"dummy-job-name": {
			"TestNodeSample":    "testdata/schema-testnode.json",
			"TestServiceSample": "testdata/schema-testservice.json",
			"TestPodSample":     "testdata/schema-testpod.json", // this file doesn't exist, I just want to test with 2 missing types
		},
	}
	err = MatchEntities(i.Entities, jobMetrics, "testdata")

	assert.Contains(t, err.Error(), "mandatory types were not found: ")
	assert.Contains(t, err.Error(), "TestServiceSample, ")
	assert.Contains(t, err.Error(), "TestPodSample, ")
}

func TestErrorValidatingTestNode(t *testing.T) {
	c := readTestInput(t, "testdata/input-invalid-testnode.json")
	i, err := integration.New("e2e", "0.0.0")
	require.NoError(t, err)

	err = json.Unmarshal(c, i)
	require.NoError(t, err)

	err = MatchEntities(i.Entities, s, "testdata")
	assert.Contains(t, err.Error(), "test-node:node1-dsn.compute.internal TestNodeSample")
	assert.Contains(t, err.Error(), "(root): capacity is required")
	assert.Contains(t, err.Error(), "test-node:node2-dsn.compute.internal TestNodeSample")
	assert.Contains(t, err.Error(), "cpuUsedCores: Invalid type. Expected: number, given: string")
}

func readTestInput(t *testing.T, filepath string) []byte {
	f, err := os.Open(filepath)
	require.NoError(t, err)

	defer f.Close()

	c, err := ioutil.ReadAll(f)
	require.NoError(t, err)

	return c
}
