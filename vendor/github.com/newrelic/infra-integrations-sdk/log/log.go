package log

import (
	"os"

	"github.com/sirupsen/logrus"
)

// SetupLogging redirects logs to stderr and configures the log level.
func SetupLogging(verbose bool) {
	logrus.SetOutput(os.Stderr)
	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}

// New creates an already configured logger.
func New(verbose bool) *logrus.Logger {
	l := logrus.New()
	ConfigureLogger(l, verbose)

	return l
}

// ConfigureLogger configures an already created logger. Redirects logs to stderr and configures the log level.
func ConfigureLogger(logger *logrus.Logger, verbose bool) {
	logger.Out = os.Stderr
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
}

func Debug(format string, args ...interface{}) {
	logrus.Debugf(format, args...)
}

func Info(format string, args ...interface{}) {
	logrus.Infof(format, args...)
}

func Warn(format string, args ...interface{}) {
	logrus.Warnf(format, args...)
}

func Error(format string, args ...interface{}) {
	logrus.Errorf(format, args...)
}

func Fatal(err error) {
	logrus.WithError(err).Fatal("can't continue")
}
