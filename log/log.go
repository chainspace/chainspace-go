// Package log provides an interface to a global logger.
package log // import "chainspace.io/prototype/log"

import (
	"time"

	"github.com/tav/golly/process"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var root *zap.SugaredLogger

func encTime(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("[2006-01-02 15:04]"))
}

func Error(args ...interface{}) {
	root.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	root.Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	root.Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	root.Fatalf(format, args...)
}

func Info(args ...interface{}) {
	root.Info(args...)
}

func Infof(format string, args ...interface{}) {
	root.Infof(format, args...)
}

func Logger() *zap.Logger {
	return root.Desugar()
}

func init() {
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.CallerKey = ""
	cfg.EncoderConfig.EncodeTime = encTime
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncoderConfig.StacktraceKey = ""
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	root = logger.Sugar()
	process.SetExitHandler(func() {
		root.Sync()
	})
}
