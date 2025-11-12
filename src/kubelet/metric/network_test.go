package metric

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

func TestFromRawWithFallbackToDefaultInterface_UsesRaw(t *testing.T) {
	expectedRawData := definition.RawGroups{
		"node": {
			"fooNode": definition.RawMetrics{
				"name":    "",
				"rxBytes": uint64(51419684038),
			},
		},
		"network": {
			"interfaces": {
				"default": "thisIsTheDefault",
			},
		},
	}

	f := FromRawWithFallbackToDefaultInterface("rxBytes")
	valueI, err := f("node", "fooNode", expectedRawData)
	require.NoError(t, err)

	value, ok := valueI.(uint64)
	require.True(t, ok)
	assert.Equal(t, uint64(51419684038), value)
}

func TestFromRawWithFallbackToDefaultInterface_UsesFallback(t *testing.T) {
	expectedRawData := definition.RawGroups{
		"node": {
			"fooNode": definition.RawMetrics{
				"name": "",
				"interfaces": map[string]definition.RawMetrics{
					"thisIsTheDefault": {
						"rxBytes": uint64(51419684038),
						"txBytes": uint64(25630208577),
						"errors":  uint64(0),
					},
				},
			},
		},
		"network": {
			"interfaces": {
				"default": "thisIsTheDefault",
			},
		},
	}

	f := FromRawWithFallbackToDefaultInterface("rxBytes")
	valueI, err := f("node", "fooNode", expectedRawData)
	require.NoError(t, err)

	value, ok := valueI.(uint64)
	require.True(t, ok)
	assert.Equal(t, uint64(51419684038), value)
}

// TestSelectPrimaryInterface tests the heuristic interface selection logic
func TestSelectPrimaryInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		interfaces    map[string]definition.RawMetrics
		expectedIface string
		expectError   bool
		errorContains string
	}{
		{
			name: "single physical interface eth0",
			interfaces: map[string]definition.RawMetrics{
				"eth0": {"rxBytes": uint64(1000)},
			},
			expectedIface: "eth0",
			expectError:   false,
		},
		{
			name: "multiple physical interfaces - selects lowest numbered",
			interfaces: map[string]definition.RawMetrics{
				"eth1": {"rxBytes": uint64(1000)},
				"eth0": {"rxBytes": uint64(2000)},
				"eth2": {"rxBytes": uint64(3000)},
			},
			expectedIface: "eth0",
			expectError:   false,
		},
		{
			name: "ens interfaces - selects lowest numbered",
			interfaces: map[string]definition.RawMetrics{
				"ens5": {"rxBytes": uint64(1000)},
				"ens3": {"rxBytes": uint64(2000)},
			},
			expectedIface: "ens3",
			expectError:   false,
		},
		{
			name: "mixed physical interfaces - alphabetical sorting",
			interfaces: map[string]definition.RawMetrics{
				"enp0s3": {"rxBytes": uint64(1000)},
				"eth0":   {"rxBytes": uint64(2000)},
				"ens3":   {"rxBytes": uint64(3000)},
				"eno1":   {"rxBytes": uint64(4000)},
			},
			expectedIface: "eno1",
			expectError:   false,
		},
		{
			name: "physical interface with CNI interfaces - filters CNI",
			interfaces: map[string]definition.RawMetrics{
				"eth0":    {"rxBytes": uint64(1000)},
				"veth123": {"rxBytes": uint64(2000)},
				"cali456": {"rxBytes": uint64(3000)},
				"azv789":  {"rxBytes": uint64(4000)},
			},
			expectedIface: "eth0",
			expectError:   false,
		},
		{
			name: "OKE scenario - ens interfaces with oci CNI",
			interfaces: map[string]definition.RawMetrics{
				"ens3":     {"rxBytes": uint64(1000)},
				"ens5":     {"rxBytes": uint64(2000)},
				"oci12345": {"rxBytes": uint64(3000)},
				"oci67890": {"rxBytes": uint64(4000)},
			},
			expectedIface: "ens3",
			expectError:   false,
		},
		{
			name: "EKS scenario - ens interfaces with eni CNI",
			interfaces: map[string]definition.RawMetrics{
				"ens5":   {"rxBytes": uint64(1000)},
				"ens6":   {"rxBytes": uint64(2000)},
				"ens7":   {"rxBytes": uint64(3000)},
				"eni123": {"rxBytes": uint64(4000)},
				"eni456": {"rxBytes": uint64(5000)},
			},
			expectedIface: "ens5",
			expectError:   false,
		},
		{
			name: "AKS scenario - eth0 with azv CNI",
			interfaces: map[string]definition.RawMetrics{
				"eth0":   {"rxBytes": uint64(1000)},
				"azv123": {"rxBytes": uint64(2000)},
				"azv456": {"rxBytes": uint64(3000)},
			},
			expectedIface: "eth0",
			expectError:   false,
		},
		{
			name: "only CNI interfaces - no physical interfaces",
			interfaces: map[string]definition.RawMetrics{
				"veth123": {"rxBytes": uint64(1000)},
				"cali456": {"rxBytes": uint64(2000)},
				"eni789":  {"rxBytes": uint64(3000)},
			},
			expectError:   true,
			errorContains: "no physical network interfaces found",
		},
		{
			name: "only non-standard interfaces - loopback and tunnels",
			interfaces: map[string]definition.RawMetrics{
				"lo":    {"rxBytes": uint64(1000)},
				"tun0":  {"rxBytes": uint64(2000)},
				"wlan0": {"rxBytes": uint64(3000)},
			},
			expectError:   true,
			errorContains: "no physical network interfaces found",
		},
		{
			name:          "empty interfaces map",
			interfaces:    map[string]definition.RawMetrics{},
			expectError:   true,
			errorContains: "no physical network interfaces found",
		},
		{
			name: "CNI patterns - docker bridge",
			interfaces: map[string]definition.RawMetrics{
				"eth0":    {"rxBytes": uint64(1000)},
				"docker0": {"rxBytes": uint64(2000)},
				"br-123":  {"rxBytes": uint64(3000)},
			},
			expectedIface: "eth0",
			expectError:   false,
		},
		{
			name: "CNI patterns - lxc and pod interfaces",
			interfaces: map[string]definition.RawMetrics{
				"ens3":    {"rxBytes": uint64(1000)},
				"lxcbr0":  {"rxBytes": uint64(2000)},
				"pod-123": {"rxBytes": uint64(3000)},
			},
			expectedIface: "ens3",
			expectError:   false,
		},
		{
			name: "CNI patterns - cni prefix",
			interfaces: map[string]definition.RawMetrics{
				"eth0":   {"rxBytes": uint64(1000)},
				"cni0":   {"rxBytes": uint64(2000)},
				"cni123": {"rxBytes": uint64(3000)},
			},
			expectedIface: "eth0",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := selectPrimaryInterface(tt.interfaces)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedIface, result)
			}
		})
	}
}

// TestGetMetricFromInterface tests extracting metrics from a specific interface
func TestGetMetricFromInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		interfaceName string
		metricKey     string
		rawMetrics    definition.RawMetrics
		expectedValue definition.FetchedValue
		expectError   bool
		errorContains string
	}{
		{
			name:          "successfully get metric from interface",
			interfaceName: "eth0",
			metricKey:     "rxBytes",
			rawMetrics: definition.RawMetrics{
				"interfaces": map[string]definition.RawMetrics{
					"eth0": {
						"rxBytes": uint64(123456),
						"txBytes": uint64(654321),
					},
				},
			},
			expectedValue: uint64(123456),
			expectError:   false,
		},
		{
			name:          "get metric from ens interface",
			interfaceName: "ens3",
			metricKey:     "txBytes",
			rawMetrics: definition.RawMetrics{
				"interfaces": map[string]definition.RawMetrics{
					"ens3": {
						"rxBytes": uint64(111111),
						"txBytes": uint64(222222),
					},
					"ens5": {
						"rxBytes": uint64(333333),
						"txBytes": uint64(444444),
					},
				},
			},
			expectedValue: uint64(222222),
			expectError:   false,
		},
		{
			name:          "interface exists but metric doesn't",
			interfaceName: "eth0",
			metricKey:     "rxErrors",
			rawMetrics: definition.RawMetrics{
				"interfaces": map[string]definition.RawMetrics{
					"eth0": {
						"rxBytes": uint64(123456),
					},
				},
			},
			expectError:   true,
			errorContains: "metric rxErrors not found for interface eth0",
		},
		{
			name:          "interface doesn't exist",
			interfaceName: "eth1",
			metricKey:     "rxBytes",
			rawMetrics: definition.RawMetrics{
				"interfaces": map[string]definition.RawMetrics{
					"eth0": {
						"rxBytes": uint64(123456),
					},
				},
			},
			expectError:   true,
			errorContains: "interface eth1 not found",
		},
		{
			name:          "no interfaces key in raw metrics",
			interfaceName: "eth0",
			metricKey:     "rxBytes",
			rawMetrics: definition.RawMetrics{
				"someOtherKey": "value",
			},
			expectError:   true,
			errorContains: "interfaces metrics not found",
		},
		{
			name:          "interfaces key has wrong type",
			interfaceName: "eth0",
			metricKey:     "rxBytes",
			rawMetrics: definition.RawMetrics{
				"interfaces": "wrong_type",
			},
			expectError:   true,
			errorContains: "wrong format for interfaces metrics",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := getMetricFromInterface(tt.interfaceName, tt.metricKey, tt.rawMetrics)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}
		})
	}
}

// TestFromRawWithFallbackToDefaultInterface_ComprehensiveFallback tests the complete 3-tier fallback logic
func TestFromRawWithFallbackToDefaultInterface_ComprehensiveFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		metricKey     string
		groups        definition.RawGroups
		groupLabel    string
		entityID      string
		expectedValue definition.FetchedValue
		expectError   bool
		errorContains string
	}{
		{
			name:       "step 3 - routing table interface not in stats, fallback to heuristic",
			metricKey:  "rxBytes",
			groupLabel: "node",
			entityID:   "node1",
			groups: definition.RawGroups{
				"node": {
					"node1": {
						// No top-level rxBytes
						"interfaces": map[string]definition.RawMetrics{
							"ens3": {
								"rxBytes": uint64(111111),
							},
							"ens5": {
								"rxBytes": uint64(222222),
							},
						},
					},
				},
				"network": {
					"interfaces": {
						"default": "eth0", // Routing table says eth0, but no stats for eth0
					},
				},
			},
			expectedValue: uint64(111111), // Falls back to heuristic (ens3 is lowest)
			expectError:   false,
		},
		{
			name:       "step 3 - no routing table, use heuristic",
			metricKey:  "txBytes",
			groupLabel: "node",
			entityID:   "node1",
			groups: definition.RawGroups{
				"node": {
					"node1": {
						// No top-level txBytes
						"interfaces": map[string]definition.RawMetrics{
							"eth1": {
								"txBytes": uint64(111111),
							},
							"eth0": {
								"txBytes": uint64(222222),
							},
							"veth123": {
								"txBytes": uint64(333333),
							},
						},
					},
				},
				// No network group at all
			},
			expectedValue: uint64(222222), // Uses heuristic (eth0 is lowest, veth filtered)
			expectError:   false,
		},
		{
			name:       "pod with top-level stats - normal pod scenario",
			metricKey:  "rxBytes",
			groupLabel: "pod",
			entityID:   "default_nginx-pod_abc123",
			groups: definition.RawGroups{
				"pod": {
					"default_nginx-pod_abc123": {
						"rxBytes": uint64(555555), // Pods typically have top-level
						"interfaces": map[string]definition.RawMetrics{
							"eth0": {
								"rxBytes": uint64(555555),
							},
						},
					},
				},
			},
			expectedValue: uint64(555555),
			expectError:   false,
		},
		{
			name:       "pod without top-level stats - hostNetwork pod with routing table",
			metricKey:  "rxBytes",
			groupLabel: "pod",
			entityID:   "kube-system_kube-proxy-xyz",
			groups: definition.RawGroups{
				"pod": {
					"kube-system_kube-proxy-xyz": {
						// No top-level rxBytes (hostNetwork pod)
						"interfaces": map[string]definition.RawMetrics{
							"ens3": {
								"rxBytes": uint64(777777),
							},
							"ens5": {
								"rxBytes": uint64(888888),
							},
							"oci123": {
								"rxBytes": uint64(999999),
							},
						},
					},
				},
				"network": {
					"interfaces": {
						"default": "ens3",
					},
				},
			},
			expectedValue: uint64(777777), // Uses routing table (ens3)
			expectError:   false,
		},
		{
			name:       "pod without top-level stats - fallback to heuristic",
			metricKey:  "txBytes",
			groupLabel: "pod",
			entityID:   "kube-system_csi-driver-xyz",
			groups: definition.RawGroups{
				"pod": {
					"kube-system_csi-driver-xyz": {
						// No top-level txBytes (hostNetwork pod)
						"interfaces": map[string]definition.RawMetrics{
							"eth0": {
								"txBytes": uint64(111111),
							},
							"docker0": {
								"txBytes": uint64(222222),
							},
						},
					},
				},
				// No network group - falls back to heuristic
			},
			expectedValue: uint64(111111), // Uses heuristic (eth0, filters docker0)
			expectError:   false,
		},
		{
			name:       "error - group not found",
			metricKey:  "rxBytes",
			groupLabel: "node",
			entityID:   "node1",
			groups: definition.RawGroups{
				"pod": { // Wrong group
					"pod1": {},
				},
			},
			expectError:   true,
			errorContains: "group not found",
		},
		{
			name:       "error - entity not found",
			metricKey:  "rxBytes",
			groupLabel: "node",
			entityID:   "node2",
			groups: definition.RawGroups{
				"node": {
					"node1": {}, // Wrong entity
				},
			},
			expectError:   true,
			errorContains: "entity not found",
		},
		{
			name:       "error - no interfaces at all",
			metricKey:  "rxBytes",
			groupLabel: "node",
			entityID:   "node1",
			groups: definition.RawGroups{
				"node": {
					"node1": {
						// No top-level, no interfaces
					},
				},
			},
			expectError:   true,
			errorContains: "interfaces metrics not found",
		},
		{
			name:       "error - only CNI interfaces, no physical",
			metricKey:  "rxBytes",
			groupLabel: "node",
			entityID:   "node1",
			groups: definition.RawGroups{
				"node": {
					"node1": {
						"interfaces": map[string]definition.RawMetrics{
							"veth123": {
								"rxBytes": uint64(111111),
							},
							"cali456": {
								"rxBytes": uint64(222222),
							},
						},
					},
				},
			},
			expectError:   true,
			errorContains: "no physical network interfaces found",
		},
		{
			name:       "error - interfaces wrong format",
			metricKey:  "rxBytes",
			groupLabel: "node",
			entityID:   "node1",
			groups: definition.RawGroups{
				"node": {
					"node1": {
						"interfaces": "wrong_type",
					},
				},
			},
			expectError:   true,
			errorContains: "wrong format for interfaces metrics",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fetchFunc := FromRawWithFallbackToDefaultInterface(tt.metricKey)
			result, err := fetchFunc(tt.groupLabel, tt.entityID, tt.groups)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}
		})
	}
}

// TestGetDefaultInterface tests reading the default interface from network group
func TestGetDefaultInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		groups        definition.RawGroups
		expectedIface string
		expectError   bool
		errorContains string
	}{
		{
			name: "successfully get default interface",
			groups: definition.RawGroups{
				"network": {
					"interfaces": {
						"default": "eth0",
					},
				},
			},
			expectedIface: "eth0",
			expectError:   false,
		},
		{
			name: "default interface is ens3",
			groups: definition.RawGroups{
				"network": {
					"interfaces": {
						"default": "ens3",
					},
				},
			},
			expectedIface: "ens3",
			expectError:   false,
		},
		{
			name:          "network group not found",
			groups:        definition.RawGroups{},
			expectError:   true,
			errorContains: "network group not found",
		},
		{
			name: "network interfaces attribute not found",
			groups: definition.RawGroups{
				"network": {
					"someOtherKey": {},
				},
			},
			expectError:   true,
			errorContains: "network interfaces attribute not found",
		},
		{
			name: "default interface not found",
			groups: definition.RawGroups{
				"network": {
					"interfaces": {
						"someOtherKey": "value",
					},
				},
			},
			expectError:   true,
			errorContains: "default interface not found",
		},
		{
			name: "default interface has wrong type",
			groups: definition.RawGroups{
				"network": {
					"interfaces": {
						"default": 123, // Not a string
					},
				},
			},
			expectError:   true,
			errorContains: "default interface is not a valid interface name",
		},
		{
			name: "default interface is empty string",
			groups: definition.RawGroups{
				"network": {
					"interfaces": {
						"default": "",
					},
				},
			},
			expectError:   true,
			errorContains: "default interface not set",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := getDefaultInterface(tt.groups)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedIface, result)
			}
		})
	}
}
