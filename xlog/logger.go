package log

import (
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
	level     level
	parfields []Field
	text      string
	time      time.Time
}

// Logger encapsulates the state of a logger with custom fields.
type Logger struct {
	fields []Field
}

func (l *Logger) log(lvl level, text string, fields []Field) {
	entry := entry{fields, lvl, l.fields, text, time.Now().UTC()}
	textLog(entry)
	// addNetEntry(entry)
}

// Debug logs the given text and fields at DebugLevel.
func (l *Logger) Debug(text string, fields ...Field) {
	l.log(DebugLevel, text, fields)
}

// Error logs the given text and fields at ErrorLevel.
func (l *Logger) Error(text string, fields ...Field) {
	l.log(ErrorLevel, text, fields)
}

// Fatal logs the given text and fields at FatalLevel.
func (l *Logger) Fatal(text string, fields ...Field) {
	l.log(FatalLevel, text, fields)
	process.Exit(1)
}

// Info logs the given text and fields at InfoLevel.
func (l *Logger) Info(text string, fields ...Field) {
	l.log(InfoLevel, text, fields)
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
