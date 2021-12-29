// Package logutil defines premade nil and debug loggers, useful for testing.
package logutil

import (
	"io"

	"github.com/sirupsen/logrus"
)

// Discard is a logrus.Logger instance configured to not log anything.
var Discard = logrus.New()

// Debug is a logrus.Logger instance configured to log in logrus.DebugLevel.
var Debug = logrus.New()

func init() {
	Discard.SetOutput(io.Discard)
	// Set level to panic might save a few cycles if we don't even attempt to write to io.Discard.
	Discard.SetLevel(logrus.PanicLevel)

	Debug.SetLevel(logrus.DebugLevel)
}
