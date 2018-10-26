package log // import "chainspace.io/prototype/internal/log"

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/tav/golly/process"
)

var bufpool = sync.Pool{
	New: func() interface{} {
		return &buffer{}
	},
}

type buffer struct {
	buf []byte
}

func (b *buffer) release() {
	b.buf = b.buf[:0]
	bufpool.Put(b)
}

type entry struct {
	fields    []Field
	level     Level
	parfields []Field
	text      string
	time      time.Time
}

// Logger encapsulates the state of a logger with custom fields.
type Logger struct {
	fields []Field
}

func (l *Logger) log(lvl Level, text string, fields []Field) {
	entry := entry{fields, lvl, l.fields, text, time.Now().UTC()}
	textLog(entry)
	netLog(entry)
}

func (l *Logger) stacktrace(text string) {
	buf := []byte(text)
	buf = append(buf, '\n', '\n')
	pcs := make([]uintptr, 64)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(3, pcs)])
	for frame, more := frames.Next(); more; frame, more = frames.Next() {
		buf = append(buf, '\t')
		buf = append(buf, frame.Function...)
		buf = append(buf, '(', ')', '\n', '\t', '\t')
		buf = append(buf, frame.File...)
		buf = append(buf, ':')
		buf = strconv.AppendInt(buf, int64(frame.Line), 10)
		buf = append(buf, '\n')
	}
	l.log(StackTraceLevel, string(buf), nil)
}

// Debug logs the given text and fields at DebugLevel.
func (l *Logger) Debug(text string, fields ...Field) {
	l.log(DebugLevel, text, fields)
}

// Debugf formats similarly to Printf and logs the resulting output at the
// DebugLevel.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DebugLevel, fmt.Sprintf(format, args...), nil)
}

// Error logs the given text and fields at ErrorLevel.
func (l *Logger) Error(text string, fields ...Field) {
	l.log(ErrorLevel, text, fields)
}

// Errorf formats similarly to Printf and logs the resulting output at the
// ErrorLevel.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...), nil)
}

// Fatal logs the given text and fields at FatalLevel.
func (l *Logger) Fatal(text string, fields ...Field) {
	l.log(FatalLevel, text, fields)
	process.Exit(1)
}

// Fatalf formats similarly to Printf and logs the resulting output at the
// FatalLevel.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(FatalLevel, fmt.Sprintf(format, args...), nil)
}

// Info logs the given text and fields at InfoLevel.
func (l *Logger) Info(text string, fields ...Field) {
	l.log(InfoLevel, text, fields)
}

// Infof formats similarly to Printf and logs the resulting output at the
// InfoLevel.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(InfoLevel, fmt.Sprintf(format, args...), nil)
}

// StackTrace logs the given text at StackTraceLevel along with the stacktrace.
func (l *Logger) StackTrace(text string) {
	l.stacktrace(text)
}

// Warn logs the given text and fields at WarnLevel.
func (l *Logger) Warn(text string, fields ...Field) {
	l.log(WarnLevel, text, fields)
}

// Warnf formats similarly to Printf and logs the resulting output at the
// WarnLevel.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WarnLevel, fmt.Sprintf(format, args...), nil)
}

// With returns a new logger that comes preset with the given fields.
func (l *Logger) With(fields ...Field) *Logger {
	if l.fields == nil {
		return &Logger{fields}
	}
	f := make([]Field, len(l.fields)+len(fields))
	copy(f, l.fields)
	copy(f[len(l.fields):], fields)
	return &Logger{f}
}
