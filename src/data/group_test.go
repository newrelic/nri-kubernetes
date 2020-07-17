package data

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestErrorGroup(t *testing.T) {
	err := ErrorGroup{
		Recoverable: true,
		Errors:      []error{errors.New("err1"), errors.New("err2"), errors.New("err3")},
	}

	err.Append(errors.New("err4"), errors.New("err5"), errors.New("err6"))

	assert.Equal(t, "Recoverable error group: err1, err2, err3, err4, err5, err6", err.String())
}
