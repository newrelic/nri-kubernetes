package timer

import (
	"time"

	"github.com/sirupsen/logrus"
)

// Track measures time, which elapsed since provided start time
func Track(start time.Time, name string, logger *logrus.Logger) {
	elapsed := time.Since(start)
	logger.Debugf("%s took %s", name, elapsed)
}
