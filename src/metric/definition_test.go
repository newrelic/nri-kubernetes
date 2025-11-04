package metric

import (
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromNano(t *testing.T) {
	v, err := fromNano(uint64(123456789))
	assert.Equal(t, 0.123456789, v)
	assert.NoError(t, err)

	v, err = fromNano(123456789)
	assert.Nil(t, v)
	assert.Error(t, err)

	v, err = fromNano("not-valid")
	assert.Nil(t, v)
	assert.Error(t, err)
}

func TestFromNanoToMilli(t *testing.T) {
	v, err := fromNanoToMilli(uint64(123456789))
	assert.Equal(t, 123.456789, v)
	assert.NoError(t, err)

	v, err = fromNano(123456789)
	assert.Nil(t, v)
	assert.Error(t, err)

	v, err = fromNano("not-valid")
	assert.Nil(t, v)
	assert.Error(t, err)
}

func TestToTimestap(t *testing.T) {
	t1, _ := time.Parse(time.RFC3339, "2018-02-14T16:26:33Z")
	v, err := toTimestamp(t1)
	assert.Equal(t, int64(1518625593), v)
	assert.NoError(t, err)

	t2, _ := time.Parse(time.RFC3339, "2016-10-21T00:45:12Z")
	v, err = toTimestamp(t2)
	assert.Equal(t, int64(1477010712), v)
	assert.NoError(t, err)
}

func TestToNumericBoolean(t *testing.T) {
	v, err := toNumericBoolean(1)
	assert.Equal(t, 1, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean(0)
	assert.Equal(t, 0, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean(true)
	assert.Equal(t, 1, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean(false)
	assert.Equal(t, 0, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean("true")
	assert.Equal(t, 1, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean("false")
	assert.Equal(t, 0, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean("True")
	assert.Equal(t, 1, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean("False")
	assert.Equal(t, 0, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean("unknown")
	assert.Equal(t, -1, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean("invalid")
	assert.Nil(t, v)
	assert.EqualError(t, err, "value 'invalid' can not be converted to numeric boolean")
}

func TestToCores(t *testing.T) {
	v, err := toCores(100)
	assert.Equal(t, float64(0.1), v)
	assert.NoError(t, err)

	v, err = toCores(int64(1000))
	assert.Equal(t, float64(1), v)
	assert.NoError(t, err)
}

func TestComputePercentage(t *testing.T) {
	v, err := computePercentage(3, 5)
	assert.Equal(t, float64(60.0), v)
	assert.NoError(t, err)

	v, err = computePercentage(3, 0)
	assert.EqualError(t, err, "division by zero")

	v, err = computePercentage(3, float64(0))
	assert.EqualError(t, err, "division by zero")

	v, err = computePercentage(3, uint64(0))
	assert.EqualError(t, err, "division by zero")
}

func TestSubtract(t *testing.T) {
	left := definition.FetchFunc(func(_, _ string, _ definition.RawGroups) (definition.FetchedValue, error) {
		return prometheus.GaugeValue(10), nil
	})

	right := definition.FetchFunc(func(_, _ string, _ definition.RawGroups) (definition.FetchedValue, error) {
		return prometheus.GaugeValue(5), nil
	})

	sub := Subtract(definition.Transform(left, fromPrometheusNumeric), definition.Transform(right, fromPrometheusNumeric))
	result, err := sub("", "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, result, float64(5))
}

func TestUtilization(t *testing.T) {
	raw := definition.RawGroups{
		"group1": {
			"entity1": {
				"dividend": uint64(10),
				"divisor":  uint64(20),
			},
			"entity2": {
				"dividend": float64(10),
				"divisor":  float64(20),
			},
			"entity3": {
				"dividend": 10,
				"divisor":  20,
			},
			"entity4": {
				"dividend": definition.FetchedValues{
					"metric1": definition.FetchedValue(float64(10)),
				},
				"divisor": float64(20),
			},
			"entity5": {
				"dividend": prometheus.GaugeValue(10),
				"divisor":  prometheus.GaugeValue(20),
			},
		},
	}

	for v := range raw["group1"] {
		value, err := toUtilization(definition.FromRaw("dividend"), definition.FromRaw("divisor"))("group1", v, raw)
		assert.NoError(t, err)
		assert.NotNil(t, value)
		assert.Equal(t, float64(50), value)
	}
}

func TestUtilizationNotSupported(t *testing.T) {
	raw := definition.RawGroups{
		"group1": {
			"entity1": {
				"dividend": definition.FetchedValues{},
				"divisor":  float64(20),
			},
			"entity2": {
				"dividend": definition.FetchedValues{
					"metric1": definition.FetchedValue(float64(10)),
					"metric2": definition.FetchedValue(float64(10)),
				},
				"divisor": float64(20),
			},
			"entity3": {
				"dividend": "15",
				"divisor":  float64(20),
			},
		},
	}

	for v := range raw["group1"] {
		value, err := toUtilization(definition.FromRaw("dividend"), definition.FromRaw("divisor"))("group1", v, raw)
		assert.Error(t, err)
		assert.Nil(t, value)
	}
}

func TestFetchIfMissing(t *testing.T) {
	valueA := float64(1)
	valueB := float64(2)
	raw := definition.RawGroups{
		"group": {
			"entity": {
				"a": valueA,
				"b": valueB,
			},
		},
	}

	emptyExpected, err := fetchIfMissing(definition.FromRaw("a"), definition.FromRaw("b"))("group", "entity", raw)
	assert.NoError(t, err)
	assert.Empty(t, emptyExpected, "No value should be fetched as main value is present")

	valueExpected, err := fetchIfMissing(definition.FromRaw("a"), definition.FromRaw("c"))("group", "entity", raw)
	assert.NoError(t, err)
	assert.Equal(t, valueA, valueExpected)
}

func TestMetricSetTypeGuesserWithCustomGroup(t *testing.T) {
	t.Parallel()

	expected := "K8sCustomSample"
	testCases := []struct {
		groupLabel string
	}{
		{groupLabel: "replicaset"},
		{groupLabel: "api-server"},
		{groupLabel: "controller-manager"},
		{groupLabel: "-controller-manager-"},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.groupLabel, func(t *testing.T) {
			t.Parallel()

			guess, err := metricSetTypeGuesserWithCustomGroup("custom")(testCase.groupLabel)
			assert.NoError(t, err)
			assert.Equal(t, expected, guess)
		})
	}
}

func Test_filterCpuUsedCores(t *testing.T) { //nolint: funlen
	t.Parallel()
	type args struct {
		fetchedValue definition.FetchedValue
		groupLabel   string
		entityID     string
		groups       definition.RawGroups
	}
	tests := []struct {
		name    string
		args    args
		want    definition.FilteredValue
		wantErr string
	}{
		{
			name: "InvalidFetchedValueType",
			args: args{
				fetchedValue: 21412412,
				groupLabel:   "dummyLabel",
				entityID:     "entity_id_1",
				groups: definition.RawGroups{
					"test": {
						"entity_id_1": definition.RawMetrics{
							"raw_metric_name_1": "dummy_val",
						},
					},
				},
			},
			want:    nil,
			wantErr: "fetchedValue must be of type float64",
		},
		{
			name: "GroupLabelNotFound",
			args: args{
				fetchedValue: 2.09,
				groupLabel:   "dummyLabel",
				entityID:     "entity_id_1",
				groups: definition.RawGroups{
					"test": {
						"entity_id_1": definition.RawMetrics{
							"raw_metric_name_1": "dummy_val",
						},
					},
				},
			},
			want:    nil,
			wantErr: "group label not found",
		},
		{
			name: "GroupEntityNotFound",
			args: args{
				fetchedValue: 2.09,
				groupLabel:   "test",
				entityID:     "dummyEntity",
				groups: definition.RawGroups{
					"test": {
						"entity_id_1": definition.RawMetrics{
							"raw_metric_name_1": "dummy_val",
						},
					},
				},
			},
			want:    nil,
			wantErr: "entity Id not found",
		},
		{
			name: "CpuLimitCoresNotFound",
			args: args{
				fetchedValue: 21.434,
				groupLabel:   "test",
				entityID:     "entity_id_1",
				groups: definition.RawGroups{
					"test": {
						"entity_id_1": definition.RawMetrics{
							"raw_metric_name_1": "dummy_val",
						},
					},
				},
			},
			want:    21.434,
			wantErr: "",
		},
		{
			name: "CpuLimitCoresTransformError",
			args: args{
				fetchedValue: 2.09,
				groupLabel:   "test",
				entityID:     "entity_id_1",
				groups: definition.RawGroups{
					"test": {
						"entity_id_1": definition.RawMetrics{
							"cpuLimitCores": "dummy_val",
						},
					},
				},
			},
			want:    nil,
			wantErr: "error transforming to cores",
		},
		{
			name: "ImpossiblyHighCpuCoresError",
			args: args{
				fetchedValue: 2141241241241113445.121,
				groupLabel:   "test",
				entityID:     "entity_id_1",
				groups: definition.RawGroups{
					"test": {
						"entity_id_1": definition.RawMetrics{
							"cpuLimitCores": 200,
						},
					},
				},
			},
			want:    nil,
			wantErr: "impossibly high value received from kubelet for cpuUsedCoresVal",
		},
		{
			name: "ValidCpuUsedCoresValue",
			args: args{
				fetchedValue: 2.09,
				groupLabel:   "test",
				entityID:     "entity_id_1",
				groups: definition.RawGroups{
					"test": {
						"entity_id_1": definition.RawMetrics{
							"cpuLimitCores": 8000,
						},
					},
				},
			},
			want:    2.09,
			wantErr: "",
		},
	}
	for _, testCase := range tests {
		tt := testCase
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := filterCPUUsedCores(tt.args.fetchedValue, tt.args.groupLabel, tt.args.entityID, tt.args.groups)
			if len(tt.wantErr) > 0 {
				assert.EqualErrorf(t, err, tt.wantErr, "expected %s, got %s", tt.wantErr, err.Error())
			} else {
				assert.Nilf(t, err, "expected nil error")
			}

			assert.Equalf(t, tt.want, got, "filterCPUUsedCores(%v, %v, %v, %v)", tt.args.fetchedValue, tt.args.groupLabel, tt.args.entityID, tt.args.groups)
		})
	}
}

// TestEndpointSpecs_KSM_v2_13_Data tests that the EndpointSpecs in definition.go
// work with KSM v2.13 data format (backward compatibility).
// KSM v2.13 provides: kube_endpoint_address_available and kube_endpoint_address_not_ready
// When multiple specs have the same name, the system tries each one until one succeeds (if Optional).
func TestEndpointSpecs_KSM_v2_13_Data(t *testing.T) {
	// Simulated KSM v2.13 output - matches the user's actual data
	ksmV213RawData := definition.RawGroups{
		"endpoint": {
			"kube-system_kube-dns": {
				"kube_endpoint_created": prometheus.GaugeValue(1620000000),
				"kube_endpoint_address_available": []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"namespace": "kube-system",
							"endpoint":  "kube-dns",
						},
						Value: prometheus.GaugeValue(3),
					},
				},
				"kube_endpoint_address_not_ready": []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"namespace": "kube-system",
							"endpoint":  "kube-dns",
						},
						Value: prometheus.GaugeValue(0),
					},
				},
			},
			"default_kubernetes": {
				"kube_endpoint_created": prometheus.GaugeValue(1620000000),
				"kube_endpoint_address_available": []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"namespace": "default",
							"endpoint":  "kubernetes",
						},
						Value: prometheus.GaugeValue(1),
					},
				},
				"kube_endpoint_address_not_ready": []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"namespace": "default",
							"endpoint":  "kubernetes",
						},
						Value: prometheus.GaugeValue(0),
					},
				},
			},
			"kube-system_k8s.io-minikube-hostpath": {
				"kube_endpoint_created": prometheus.GaugeValue(1620000000),
				"kube_endpoint_address_available": []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"namespace": "kube-system",
							"endpoint":  "k8s.io-minikube-hostpath",
						},
						Value: prometheus.GaugeValue(0),
					},
				},
				"kube_endpoint_address_not_ready": []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"namespace": "kube-system",
							"endpoint":  "k8s.io-minikube-hostpath",
						},
						Value: prometheus.GaugeValue(0),
					},
				},
			},
		},
	}

	// Get the actual EndpointSpecs from definition.go
	endpointSpecs := KSMSpecs["endpoint"]

	// Find ALL addressAvailable and addressNotReady specs (there should be 2 of each for backward compatibility)
	var addressAvailableSpecs, addressNotReadySpecs []definition.Spec
	for i := range endpointSpecs.Specs {
		if endpointSpecs.Specs[i].Name == "addressAvailable" {
			addressAvailableSpecs = append(addressAvailableSpecs, endpointSpecs.Specs[i])
		}
		if endpointSpecs.Specs[i].Name == "addressNotReady" {
			addressNotReadySpecs = append(addressNotReadySpecs, endpointSpecs.Specs[i])
		}
	}

	require.Len(t, addressAvailableSpecs, 2, "Should have exactly 2 addressAvailable specs for backward compatibility (v2.13 and v2.16)")
	require.Len(t, addressNotReadySpecs, 2, "Should have exactly 2 addressNotReady specs for backward compatibility (v2.13 and v2.16)")

	testCases := []struct {
		name             string
		entityID         string
		expectedAvail    prometheus.GaugeValue
		expectedNotReady prometheus.GaugeValue
	}{
		{
			name:             "kube-dns with 3 available, 0 not ready",
			entityID:         "kube-system_kube-dns",
			expectedAvail:    prometheus.GaugeValue(3),
			expectedNotReady: prometheus.GaugeValue(0),
		},
		{
			name:             "kubernetes with 1 available, 0 not ready",
			entityID:         "default_kubernetes",
			expectedAvail:    prometheus.GaugeValue(1),
			expectedNotReady: prometheus.GaugeValue(0),
		},
		{
			name:             "minikube-hostpath with 0 available, 0 not ready",
			entityID:         "kube-system_k8s.io-minikube-hostpath",
			expectedAvail:    prometheus.GaugeValue(0),
			expectedNotReady: prometheus.GaugeValue(0),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test addressAvailable - try ALL specs with this name (simulates what populator does)
			t.Run("addressAvailable", func(t *testing.T) {
				var fetchedValues definition.FetchedValues
				var lastErr error

				// Try each spec with name "addressAvailable" until one works (mimics populator behavior)
				for _, spec := range addressAvailableSpecs {
					result, err := spec.ValueFunc("endpoint", tc.entityID, ksmV213RawData)
					if err != nil {
						lastErr = err
						if !spec.Optional {
							require.NoError(t, err, "Non-optional spec failed")
						}
						t.Logf("Spec failed (optional, trying next): %v", err)
						continue
					}

					// Success! Found a working spec
					var ok bool
					fetchedValues, ok = result.(definition.FetchedValues)
					require.True(t, ok, "Result should be FetchedValues")
					break
				}

				// At least one spec should have succeeded
				require.NotNil(t, fetchedValues, "At least one addressAvailable spec should work with KSM v2.13 data. Last error: %v", lastErr)

				// When value is 0, result might be empty or have a 0 value
				if tc.expectedAvail == 0 && len(fetchedValues) == 0 {
					t.Logf("✓ Correctly returned empty result for 0 available addresses")
					return
				}

				require.NotEmpty(t, fetchedValues, "Should have fetched values from KSM v2.13 data")

				for metricName, val := range fetchedValues {
					gaugeValue, ok := val.(prometheus.GaugeValue)
					require.True(t, ok, "Value should be GaugeValue")
					assert.Equal(t, tc.expectedAvail, gaugeValue,
						"addressAvailable should match expected value from KSM v2.13")
					t.Logf("✓ Metric: %s = %v (expected %v)", metricName, gaugeValue, tc.expectedAvail)
				}
			})

			// Test addressNotReady - try ALL specs with this name
			t.Run("addressNotReady", func(t *testing.T) {
				var fetchedValues definition.FetchedValues
				var lastErr error

				// Try each spec with name "addressNotReady" until one works
				for _, spec := range addressNotReadySpecs {
					result, err := spec.ValueFunc("endpoint", tc.entityID, ksmV213RawData)
					if err != nil {
						lastErr = err
						if !spec.Optional {
							require.NoError(t, err, "Non-optional spec failed")
						}
						t.Logf("Spec failed (optional, trying next): %v", err)
						continue
					}

					// Success! Found a working spec
					var ok bool
					fetchedValues, ok = result.(definition.FetchedValues)
					require.True(t, ok, "Result should be FetchedValues")
					break
				}

				// At least one spec should have succeeded
				require.NotNil(t, fetchedValues, "At least one addressNotReady spec should work with KSM v2.13 data. Last error: %v", lastErr)

				// When value is 0, result might be empty or have a 0 value
				if tc.expectedNotReady == 0 && len(fetchedValues) == 0 {
					t.Logf("✓ Correctly returned empty result for 0 not ready addresses")
					return
				}

				require.NotEmpty(t, fetchedValues, "Should have fetched values from KSM v2.13 data")

				for metricName, val := range fetchedValues {
					gaugeValue, ok := val.(prometheus.GaugeValue)
					require.True(t, ok, "Value should be GaugeValue")
					assert.Equal(t, tc.expectedNotReady, gaugeValue,
						"addressNotReady should match expected value from KSM v2.13")
					t.Logf("✓ Metric: %s = %v (expected %v)", metricName, gaugeValue, tc.expectedNotReady)
				}
			})
		})
	}
}

// TestEndpointSpecs_KSM_v2_16_Data tests that the current EndpointSpecs in definition.go
// work correctly with KSM v2.16 data format.
// KSM v2.16 provides: kube_endpoint_address with detailed labels including "ready".
func TestEndpointSpecs_KSM_v2_16_Data(t *testing.T) {
	// Simulated KSM v2.16 output - has kube_endpoint_address with "ready" label
	ksmV216RawData := definition.RawGroups{
		"endpoint": {
			"kube-system_kube-dns": {
				"kube_endpoint_created": prometheus.GaugeValue(1620000000),
				"kube_endpoint_address": []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"namespace":     "kube-system",
							"endpoint":      "kube-dns",
							"port_protocol": "TCP",
							"port_number":   "53",
							"port_name":     "dns-tcp",
							"ip":            "10.244.0.2",
							"ready":         "true",
						},
						Value: prometheus.GaugeValue(1),
					},
					{
						Labels: prometheus.Labels{
							"namespace":     "kube-system",
							"endpoint":      "kube-dns",
							"port_protocol": "UDP",
							"port_number":   "53",
							"port_name":     "dns",
							"ip":            "10.244.0.2",
							"ready":         "true",
						},
						Value: prometheus.GaugeValue(1),
					},
					{
						Labels: prometheus.Labels{
							"namespace":     "kube-system",
							"endpoint":      "kube-dns",
							"port_protocol": "TCP",
							"port_number":   "9153",
							"port_name":     "metrics",
							"ip":            "10.244.0.2",
							"ready":         "true",
						},
						Value: prometheus.GaugeValue(1),
					},
				},
			},
			"default_kubernetes": {
				"kube_endpoint_created": prometheus.GaugeValue(1620000000),
				"kube_endpoint_address": []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"namespace":     "default",
							"endpoint":      "kubernetes",
							"port_protocol": "TCP",
							"port_number":   "8443",
							"port_name":     "https",
							"ip":            "192.168.49.2",
							"ready":         "true",
						},
						Value: prometheus.GaugeValue(1),
					},
				},
			},
			"kube-system_k8s.io-minikube-hostpath": {
				"kube_endpoint_created": prometheus.GaugeValue(1620000000),
				"kube_endpoint_address": []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"namespace":     "kube-system",
							"endpoint":      "k8s.io-minikube-hostpath",
							"port_protocol": "TCP",
							"port_number":   "80",
							"port_name":     "",
							"ip":            "10.244.0.20",
							"ready":         "false",
						},
						Value: prometheus.GaugeValue(1),
					},
				},
			},
		},
	}

	// Get the actual EndpointSpecs from definition.go
	endpointSpecs := KSMSpecs["endpoint"]

	// Find ALL addressAvailable and addressNotReady specs (there should be 2 of each for backward compatibility)
	var addressAvailableSpecs, addressNotReadySpecs []definition.Spec
	for i := range endpointSpecs.Specs {
		if endpointSpecs.Specs[i].Name == "addressAvailable" {
			addressAvailableSpecs = append(addressAvailableSpecs, endpointSpecs.Specs[i])
		}
		if endpointSpecs.Specs[i].Name == "addressNotReady" {
			addressNotReadySpecs = append(addressNotReadySpecs, endpointSpecs.Specs[i])
		}
	}

	require.Len(t, addressAvailableSpecs, 2, "Should have exactly 2 addressAvailable specs for backward compatibility (v2.13 and v2.16)")
	require.Len(t, addressNotReadySpecs, 2, "Should have exactly 2 addressNotReady specs for backward compatibility (v2.13 and v2.16)")

	testCases := []struct {
		name             string
		entityID         string
		expectedAvail    prometheus.GaugeValue
		expectedNotReady prometheus.GaugeValue
		description      string
	}{
		{
			name:             "kube-dns with 3 ready addresses",
			entityID:         "kube-system_kube-dns",
			expectedAvail:    prometheus.GaugeValue(3),
			expectedNotReady: prometheus.GaugeValue(0),
			description:      "kube-dns has 3 ports (TCP/53, UDP/53, TCP/9153) all ready",
		},
		{
			name:             "kubernetes with 1 ready address",
			entityID:         "default_kubernetes",
			expectedAvail:    prometheus.GaugeValue(1),
			expectedNotReady: prometheus.GaugeValue(0),
			description:      "kubernetes has 1 port (TCP/8443) ready",
		},
		{
			name:             "minikube-hostpath with 1 not ready address",
			entityID:         "kube-system_k8s.io-minikube-hostpath",
			expectedAvail:    prometheus.GaugeValue(0),
			expectedNotReady: prometheus.GaugeValue(1),
			description:      "minikube-hostpath has 1 port (TCP/80) not ready",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.description)

			// Test addressAvailable - try ALL specs with this name (simulates what populator does)
			t.Run("addressAvailable", func(t *testing.T) {
				var fetchedValues definition.FetchedValues
				var lastErr error

				// Try each spec with name "addressAvailable" until one works (mimics populator behavior)
				for _, spec := range addressAvailableSpecs {
					result, err := spec.ValueFunc("endpoint", tc.entityID, ksmV216RawData)
					if err != nil {
						lastErr = err
						if !spec.Optional {
							require.NoError(t, err, "Non-optional spec failed")
						}
						t.Logf("Spec failed (optional, trying next): %v", err)
						continue
					}

					// Success! Found a working spec
					var ok bool
					fetchedValues, ok = result.(definition.FetchedValues)
					require.True(t, ok, "Result should be FetchedValues")
					break
				}

				// At least one spec should have succeeded
				require.NotNil(t, fetchedValues, "At least one addressAvailable spec should work with KSM v2.16 data. Last error: %v", lastErr)

				// When value is 0, result might be empty or have a 0 value
				if tc.expectedAvail == 0 && len(fetchedValues) == 0 {
					t.Logf("✓ Correctly returned empty result (0 addresses with ready=true)")
					return
				}

				require.NotEmpty(t, fetchedValues, "Should have fetched values from KSM v2.16 data")

				for metricName, val := range fetchedValues {
					gaugeValue, ok := val.(prometheus.GaugeValue)
					require.True(t, ok, "Value should be GaugeValue")
					assert.Equal(t, tc.expectedAvail, gaugeValue,
						"addressAvailable count should match expected")
					t.Logf("✓ Metric: %s = %v (expected %v)", metricName, gaugeValue, tc.expectedAvail)
				}
			})

			// Test addressNotReady - try ALL specs with this name
			t.Run("addressNotReady", func(t *testing.T) {
				var fetchedValues definition.FetchedValues
				var lastErr error

				// Try each spec with name "addressNotReady" until one works
				for _, spec := range addressNotReadySpecs {
					result, err := spec.ValueFunc("endpoint", tc.entityID, ksmV216RawData)
					if err != nil {
						lastErr = err
						if !spec.Optional {
							require.NoError(t, err, "Non-optional spec failed")
						}
						t.Logf("Spec failed (optional, trying next): %v", err)
						continue
					}

					// Success! Found a working spec
					var ok bool
					fetchedValues, ok = result.(definition.FetchedValues)
					require.True(t, ok, "Result should be FetchedValues")
					break
				}

				// At least one spec should have succeeded
				require.NotNil(t, fetchedValues, "At least one addressNotReady spec should work with KSM v2.16 data. Last error: %v", lastErr)

				// When value is 0, result might be empty or have a 0 value
				if tc.expectedNotReady == 0 && len(fetchedValues) == 0 {
					t.Logf("✓ Correctly returned empty result (0 addresses with ready=false)")
					return
				}

				require.NotEmpty(t, fetchedValues, "Should have fetched values from KSM v2.16 data")

				for metricName, val := range fetchedValues {
					gaugeValue, ok := val.(prometheus.GaugeValue)
					require.True(t, ok, "Value should be GaugeValue")
					assert.Equal(t, tc.expectedNotReady, gaugeValue,
						"addressNotReady count should match expected")
					t.Logf("✓ Metric: %s = %v (expected %v)", metricName, gaugeValue, tc.expectedNotReady)
				}
			})
		})
	}
}

func Test_KSM_LabelAndAnnotationExtraction_WithKSMSpecs(t *testing.T) {
	t.Parallel()
	raw := definition.RawGroups{
		"namespace": {
			"my-namespace": {
				"kube_namespace_labels": prometheus.Metric{
					Labels: prometheus.Labels{
						"label_team": "devops",
						"label_env":  "staging",
					},
				},
				"kube_namespace_annotations": prometheus.Metric{
					Labels: prometheus.Labels{
						"annotation_owner": "alice",
					},
				},
			},
		},
		"pod": {
			"my-pod": {
				"kube_pod_labels": prometheus.Metric{
					Labels: prometheus.Labels{
						"label_app": "nginx",
						"label_env": "prod",
					},
				},
				"kube_pod_annotations": prometheus.Metric{
					Labels: prometheus.Labels{
						"annotation_owner": "bob",
					},
				},
			},
		},
	}

	getSpec := func(group, name string) definition.Spec {
		for _, spec := range KSMSpecs[group].Specs {
			if spec.Name == name {
				return spec
			}
		}
		t.Fatalf("spec %s not found for group %s", name, group)
		return definition.Spec{}
	}

	labels, err := getSpec("namespace", "label.*").ValueFunc("namespace", "my-namespace", raw)
	require.NoError(t, err)
	assert.Equal(t, definition.FetchedValues{"label.team": "devops", "label.env": "staging"}, labels)

	annotations, err := getSpec("namespace", "annotation.*").ValueFunc("namespace", "my-namespace", raw)
	require.NoError(t, err)
	assert.Equal(t, definition.FetchedValues{"annotation.owner": "alice"}, annotations)

	labels, err = getSpec("pod", "label.*").ValueFunc("pod", "my-pod", raw)
	require.NoError(t, err)
	assert.Equal(t, definition.FetchedValues{"label.app": "nginx", "label.env": "prod"}, labels)

	annotations, err = getSpec("pod", "annotation.*").ValueFunc("pod", "my-pod", raw)
	require.NoError(t, err)
	assert.Equal(t, definition.FetchedValues{"annotation.owner": "bob"}, annotations)
}
