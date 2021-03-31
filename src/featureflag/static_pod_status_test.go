package featureflag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/version"
)

func TestStaticPodStatus(t *testing.T) {
	testCases := []struct {
		name     string
		version  *version.Info
		expected bool
	}{
		{
			name:     "couldn't retrieve version",
			version:  nil,
			expected: false,
		},
		{
			name:     "major is greater",
			version:  &version.Info{Major: "2", Minor: "0"},
			expected: true,
		},
		{
			name:     "major is the same and minor is greater",
			version:  &version.Info{Major: "1", Minor: "15"},
			expected: true,
		},
		{
			name:     "major is the same and minor is less than supported",
			version:  &version.Info{Major: "1", Minor: "14"},
			expected: false,
		},
		{
			name:     "major is the same and minor has symbol",
			version:  &version.Info{Major: "1", Minor: "18+"},
			expected: true,
		},
		{
			name:     "major is the same and minor has patch",
			version:  &version.Info{Major: "1", Minor: "18.12"},
			expected: true,
		},
		{
			name:     "major is the same and minor has patch less than supported",
			version:  &version.Info{Major: "1", Minor: "13.12"},
			expected: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, StaticPodsStatus(testCase.version))
		})
	}
}
