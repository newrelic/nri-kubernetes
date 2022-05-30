package asserter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendToCopy(t *testing.T) {
	a := make([]int, 0, 10)
	b := appendToCopy(a, 1, 2, 3)
	assert.Len(t, a, 0, "original slice should not change")
	assert.Equal(t, 10, cap(a))
	assert.Equal(t, []int{1, 2, 3}, b)
	assert.Equal(t, 3, cap(b))
}
