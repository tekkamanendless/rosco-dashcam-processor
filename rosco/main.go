package rosco

import "github.com/sirupsen/logrus"

var logger *logrus.Logger

func init() {
	logger = logrus.New()
}

// SetLogLevel sets the log level for this package.
func SetLogLevel(level logrus.Level) {
	logger.SetLevel(level)
}
