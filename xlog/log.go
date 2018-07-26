// Package log provides support for structured logging.
package log // import "chainspace.io/prototype/xlog"

import (
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

// Error logs the given text and fields at ErrorLevel using the root logger.
func Error(text string, fields ...Field) {
	root.Error(text, fields...)
}

// Fatal logs the given text and fields at FatalLevel using the root logger.
func Fatal(text string, fields ...Field) {
	root.Fatal(text, fields...)
}

// Info logs the given text and fields at InfoLevel using the root logger.
func Info(text string, fields ...Field) {
	root.Info(text, fields...)
}

// SetGlobal sets the given fields on the root logger. SetGlobal is not
// threadsafe, so should be set before any goroutines that make log calls.
func SetGlobal(fields ...Field) {
	root.fields = fields
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
