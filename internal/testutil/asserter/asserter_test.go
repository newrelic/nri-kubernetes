package asserter

import (
	"testing"

	"github.com/newrelic/nri-kubernetes/v3/internal/testutil/asserter/exclude"
	"github.com/stretchr/testify/assert"
)

func TestExcludeCopy(t *testing.T) {
	a := Asserter{exclude: make([]exclude.Func, 0, 10)}
	b := a.Excluding(nil, nil, nil).Excluding(nil)
	assert.Len(t, a.exclude, 0, "original slice should not change")
	assert.Equal(t, 10, cap(a.exclude))
	assert.Equal(t, []exclude.Func{nil, nil, nil, nil}, b.exclude)
	assert.Equal(t, 4, cap(b.exclude))
}

func TestExcludeGroupsCopy(t *testing.T) {
	a := Asserter{excludedGroups: make([]string, 0, 10)}
	b := a.ExcludingGroups("g1", "g2", "g3").ExcludingGroups("g4")
	assert.Len(t, a.excludedGroups, 0, "original slice should not change")
	assert.Equal(t, 10, cap(a.excludedGroups))
	assert.Equal(t, []string{"g1", "g2", "g3", "g4"}, b.excludedGroups)
	assert.Equal(t, 4, cap(b.excludedGroups))
}
