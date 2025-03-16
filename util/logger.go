package util

import (
	"io"
	"os"

	"github.com/PooriaJ/RediDNS/config"
	"github.com/sirupsen/logrus"
)

// NewLogger creates a new logger instance
func NewLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	return logger
}

// ConfigureLogger configures the logger based on the application configuration
func ConfigureLogger(logger *logrus.Logger, cfg *config.Config) error {
	// Set log level
	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		return err
	}
	logger.SetLevel(level)

	// Set log output
	if cfg.Log.File != "" {
		file, err := os.OpenFile(cfg.Log.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}

		// Use MultiWriter to log to both file and stdout
		mw := io.MultiWriter(os.Stdout, file)
		logger.SetOutput(mw)
	}

	return nil
}
