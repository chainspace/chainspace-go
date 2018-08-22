// Package log provides support for structured logging.
package log // import "chainspace.io/prototype/log"

import (
	"fmt"
	"os"

	"github.com/tav/golly/process"
)

var (
	root = &Logger{}
)

// Debug logs the given text and fields at DebugLevel using the root logger.
func Debug(text string, fields ...Field) {
	root.Debug(text, fields...)
}

// Debugf formats similarly to Printf and logs the resulting output at the
// DebugLevel using the root logger.
func Debugf(format string, args ...interface{}) {
	root.Debug(fmt.Sprintf(format, args...))
}

// Error logs the given text and fields at ErrorLevel using the root logger.
func Error(text string, fields ...Field) {
	root.Error(text, fields...)
}

// Errorf formats similarly to Printf and logs the resulting output at the
// ErrorLevel using the root logger.
func Errorf(format string, args ...interface{}) {
	root.Error(fmt.Sprintf(format, args...))
}

// Fatal logs the given text and fields at FatalLevel using the root logger.
func Fatal(text string, fields ...Field) {
	root.Fatal(text, fields...)
}

// Fatalf formats similarly to Printf and logs the resulting output at the
// FatalLevel using the root logger.
func Fatalf(format string, args ...interface{}) {
	root.Fatal(fmt.Sprintf(format, args...))
}

// Info logs the given text and fields at InfoLevel using the root logger.
func Info(text string, fields ...Field) {
	root.Info(text, fields...)
}

// Infof formats similarly to Printf and logs the resulting output at the
// InfoLevel using the root logger.
func Infof(format string, args ...interface{}) {
	root.Info(fmt.Sprintf(format, args...))
}

// SetGlobal sets the given fields on the root logger. SetGlobal is not
// threadsafe, so should be set before any goroutines that make log calls.
func SetGlobal(fields ...Field) {
	root.fields = fields
}

// Trace logs the given text at TraceLevel along with the stacktrace using the
// root logger.
func Trace(text string) {
	root.stacktrace(text)
}

// With returns a new logger based off of the root logger that comes preset with
// the given fields.
func With(fields ...Field) *Logger {
	return root.With(fields...)
}

func init() {
	process.SetExitHandler(func() {
		os.Stderr.Sync()
		fileMu.Lock()
		if logFile != nil {
			logFile.Sync()
			logFile.Close()
		}
		fileMu.Unlock()
	})
}
