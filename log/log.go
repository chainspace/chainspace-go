// Please note: the logging levels are copied verbatim from zapcore, which is
// under the MIT License.

// Package log provides an interface to a global logger.
package log // import "chainspace.io/prototype/log"

import (
	"github.com/tav/golly/process"
	"go.uber.org/zap"
)

var root *zap.Logger

// A Level is a logging priority. Higher levels are more important.
type Level int8

// Logging levels.
const (
	// DebugLevel logs are typically voluminous, and are usually disabled in
	// production.
	DebugLevel Level = iota - 1
	// InfoLevel is the default logging priority.
	InfoLevel
	// WarnLevel logs are more important than Info, but don't need individual
	// human review.
	WarnLevel
	// ErrorLevel logs are high-priority. If an application is running smoothly,
	// it shouldn't generate any error-level logs.
	ErrorLevel
	// DPanicLevel logs are particularly important errors. In development the
	// logger panics after writing the message.
	DPanicLevel
	// PanicLevel logs a message, then panics.
	PanicLevel
	// FatalLevel logs a message, then calls os.Exit(1).
	FatalLevel
)

func Error(msg string, fields ...zap.Field) {
	root.Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	root.Fatal(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	root.Info(msg, fields...)
}

func SetGlobalFields(fields ...zap.Field) {
	root = root.With(fields...)
}

func With(fields ...zap.Field) *zap.Logger {
	return root.With(fields...)
}

func init() {
	// Flush the logs before exiting the process.
	process.SetExitHandler(func() {
		if root != nil {
			root.Sync()
		}
	})
}
