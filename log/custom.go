package log

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func buildCfg(path string, lvl Level) (*zap.Logger, error) {
	enc := zapcore.EncoderConfig{
		CallerKey:      "",
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     encTime,
		LevelKey:       "L",
		LineEnding:     zapcore.DefaultLineEnding,
		MessageKey:     "M",
		NameKey:        "N",
		StacktraceKey:  "",
		TimeKey:        "T",
	}
	cfg := zap.Config{
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    enc,
		ErrorOutputPaths: []string{path},
		Level:            zap.NewAtomicLevelAt(zapcore.Level(lvl)),
		OutputPaths:      []string{path},
	}
	return cfg.Build()
}

func encTime(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("[2006-01-02 15:04:05]"))
}

func setLogger(path string, lvl Level) error {
	l, err := buildCfg(path, lvl)
	if err != nil {
		return err
	}
	if root == nil {
		root = l
		return nil
	}
	c := l.Core()
	wrap := zap.WrapCore(func(r zapcore.Core) zapcore.Core {
		return zapcore.NewTee(r, c)
	})
	root = root.WithOptions(wrap)
	return nil
}

// InitConsoleLogger initialises a console logger configured with defaults for
// use as the root logger. If a root logger already exists, it will be tee-d
// together with the new console logger.
func InitConsoleLogger(lvl Level) error {
	return setLogger("stderr", lvl)
}

// InitFileLogger initialises a file logger configured with defaults for use as
// the root logger. If a root logger already exists, it will be tee-d together
// with the new file logger.
func InitFileLogger(path string, lvl Level) error {
	return setLogger(path, lvl)
}
