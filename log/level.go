package log

import (
	"fmt"
	"strconv"
	"strings"
)

// Logging levels.
const (
	DebugLevel Level = iota + 1
	InfoLevel
	WarnLevel
	ErrorLevel
	StackTraceLevel
	FatalLevel
)

const maxLevel = FatalLevel + 1

const (
	blue    = 34
	cyan    = 36
	magenta = 35
	red     = 31
	yellow  = 33
)

var (
	minLevel     = maxLevel
	consoleLevel = maxLevel
	fileLevel    = maxLevel
	netLevel     = maxLevel
	textLevel    = maxLevel
)

var (
	level2color  = [...]string{"", color(yellow, "DEBUG"), color(blue, "INFO"), color(cyan, "WARN"), color(red, "ERROR"), color(magenta, "STACKTRACE"), color(red, "FATAL")}
	level2string = [...]string{"", rpad("DEBUG"), rpad("INFO"), rpad("WARN"), rpad("ERROR"), rpad("STACKTRACE"), rpad("FATAL")}
)

// Level represents a logging level.
type Level int8

// MarshalYAML implements the YAML encoding interface.
func (l Level) MarshalYAML() (interface{}, error) {
	switch l {
	case 0:
		return "", nil
	case DebugLevel:
		return "debug", nil
	case ErrorLevel:
		return "error", nil
	case FatalLevel:
		return "fatal", nil
	case InfoLevel:
		return "info", nil
	case StackTraceLevel:
		return "stacktrace", nil
	case WarnLevel:
		return "warn", nil
	default:
		panic(fmt.Errorf("log: unknown level: %d", l))
	}
}

func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	case InfoLevel:
		return "INFO"
	case StackTraceLevel:
		return "STACKTRACE"
	default:
		panic(fmt.Errorf("log: unknown level: %d", l))
	}
}

// UnmarshalYAML implements the YAML decoding interface.
func (l *Level) UnmarshalYAML(unmarshal func(interface{}) error) error {
	raw := ""
	if err := unmarshal(&raw); err != nil {
		return err
	}
	switch strings.ToLower(raw) {
	case "":
	case "debug":
		*l = DebugLevel
	case "error":
		*l = ErrorLevel
	case "fatal":
		*l = FatalLevel
	case "info":
		*l = InfoLevel
	case "stacktrace":
		*l = StackTraceLevel
	case "warn":
		*l = WarnLevel
	default:
		return fmt.Errorf("log: unable to decode Level value: %q", raw)
	}
	return nil
}

func color(code int64, text string) string {
	return "\x1b[" + strconv.FormatInt(code, 10) + "m" + rpad(text) + "\x1b[0m"
}

func rpad(text string) string {
	for i := len(text); i < 12; i++ {
		text += " "
	}
	return text
}

func updateMinLevels() {
	minLevel = maxLevel
	textLevel = maxLevel
	if consoleLevel < minLevel {
		minLevel = consoleLevel
		textLevel = consoleLevel
	}
	if fileLevel < minLevel {
		minLevel = fileLevel
		textLevel = fileLevel
	}
	if netLevel < minLevel {
		minLevel = netLevel
	}
}

// AtDebug returns whether the current configuration will log at the DebugLevel.
func AtDebug() bool {
	return minLevel <= DebugLevel
}

// AtError returns whether the current configuration will log at the ErrorLevel.
func AtError() bool {
	return minLevel <= ErrorLevel
}

// AtInfo returns whether the current configuration will log at the InfoLevel.
func AtInfo() bool {
	return minLevel <= InfoLevel
}

// AtStackTrace returns whether the current configuration will log at the
// StackTraceLevel.
func AtStackTrace() bool {
	return minLevel <= StackTraceLevel
}

// AtWarn returns whether the current configuration will log at the WarnLevel.
func AtWarn() bool {
	return minLevel <= WarnLevel
}
