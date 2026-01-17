// Package log provides centralized logging for Ralph using charmbracelet/log.
package log

import (
	"os"

	"github.com/charmbracelet/log"
)

// Logger is the global logger instance.
var Logger *log.Logger

func init() {
	Logger = log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		Level:           log.InfoLevel,
	})
}

// SetLevel sets the logging level.
func SetLevel(level log.Level) {
	Logger.SetLevel(level)
}

// Debug logs a debug message.
func Debug(msg interface{}, keyvals ...interface{}) {
	Logger.Debug(msg, keyvals...)
}

// Info logs an info message.
func Info(msg interface{}, keyvals ...interface{}) {
	Logger.Info(msg, keyvals...)
}

// Warn logs a warning message.
func Warn(msg interface{}, keyvals ...interface{}) {
	Logger.Warn(msg, keyvals...)
}

// Error logs an error message.
func Error(msg interface{}, keyvals ...interface{}) {
	Logger.Error(msg, keyvals...)
}

// Fatal logs a fatal message and exits.
func Fatal(msg interface{}, keyvals ...interface{}) {
	Logger.Fatal(msg, keyvals...)
}

// CloseError logs an error from a close operation if the error is not nil.
// This is useful for handling deferred close errors.
func CloseError(resource string, err error) {
	if err != nil {
		Logger.Warn("failed to close resource", "resource", resource, "error", err)
	}
}
