// Please note: the logging levels are copied verbatim from zapcore, which is
// under the MIT License.

// Package log provides an interface to a global logger.
package log // import "chainspace.io/prototype/log"

import (
	"github.com/tav/golly/process"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

func Check(lvl Level, msg string) *zapcore.CheckedEntry {
	return root.Check(zapcore.Level(lvl), msg)
}

func Error(args ...interface{}) {
	root.Sugar().Error(args...)
}

func Errorf(format string, args ...interface{}) {
	root.Sugar().Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	root.Sugar().Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	root.Sugar().Fatalf(format, args...)
}

func Info(args ...interface{}) {
	root.Sugar().Info(args...)
}

func Infof(format string, args ...interface{}) {
	root.Sugar().Infof(format, args...)
}

// func Info(log string, fields ...zapcore.Fields) {
// 	// root.Info(args...)
// }

// func Info(ctx context.Context, log string, fields ...zapcore.Fields) {
// 	// root.Info(args...)
// }

// func Infof(format string, args ...interface{}) {
// 	root.Infof(format, args...)
// }

// GlobalLogger returns the currently set global logger.
func GlobalLogger() *zap.Logger {
	return root
}

// SetLogger sets the given logger as the global logger for the system.
func SetLogger(l *zap.Logger) {
	root = l
}

func init() {
	// Flush the logs before exiting the process.
	process.SetExitHandler(func() {
		if root != nil {
			root.Sync()
		}
	})
}
