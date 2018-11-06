package log

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	hex      = "0123456789abcdef"
	hexUpper = "0123456789ABCDEF"
	timefmt  = "2006-01-02 15:04:05"
)

var (
	consoleMu sync.Mutex
	fileMu    sync.Mutex
	logFile   *os.File
)

func textLog(e entry) {
	if textLevel > e.level {
		return
	}
	b := bufpool.Get().(*buffer)
	defer b.release()
	b.buf = append(b.buf, '[')
	b.buf = e.time.AppendFormat(b.buf, timefmt)
	b.buf = append(b.buf, ']', ' ', ' ', ' ')
	b.buf = append(b.buf, level2color[e.level]...)
	b.buf = append(b.buf, e.text...)
	b.buf = append(b.buf, '\n')
	if len(e.parfields) > 0 || len(e.fields) > 0 {
		b.buf = append(b.buf, "                                  "...)
		if len(e.parfields) > 0 {
			writeTextFields(b, e.parfields)
		}
		if len(e.fields) > 0 {
			writeTextFields(b, e.fields)
		}
		b.buf = append(b.buf, '\n')
	}
	if consoleLevel <= e.level {
		consoleMu.Lock()
		os.Stderr.Write(b.buf)
		consoleMu.Unlock()
	}

	fileMu.Lock()
	if logFile != nil && fileLevel <= e.level {
		logFile.Write(b.buf)
	}
	fileMu.Unlock()

}

// Adapted from the MIT-licensed zap.
func writeTextByte(b *buffer, v byte) bool {
	if v >= utf8.RuneSelf {
		return false
	}
	if 0x20 <= v && v != '\\' && v != '"' {
		b.buf = append(b.buf, v)
		return true
	}
	switch v {
	case '\\', '"':
		b.buf = append(b.buf, '\\')
		b.buf = append(b.buf, v)
	case '\n':
		b.buf = append(b.buf, '\\')
		b.buf = append(b.buf, 'n')
	case '\r':
		b.buf = append(b.buf, '\\')
		b.buf = append(b.buf, 'r')
	case '\t':
		b.buf = append(b.buf, '\\')
		b.buf = append(b.buf, 't')
	default:
		b.buf = append(b.buf, `\u00`...)
		b.buf = append(b.buf, hex[v>>4])
		b.buf = append(b.buf, hex[v&0x0f])
	}
	return true
}

// Adapted from the MIT-licensed zap.
func writeTextBytes(b *buffer, s []byte) {
	for i := 0; i < len(s); {
		if writeTextByte(b, s[i]) {
			i++
			continue
		}
		r, size := utf8.DecodeRune(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.buf = append(b.buf, `\ufffd`...)
			i++
			continue
		}
		b.buf = append(b.buf, s[i:i+size]...)
		i += size
	}
}

func writeTextDigest(b *buffer, hash []byte) {
	for _, v := range hash[:6] {
		b.buf = append(b.buf, hexUpper[v>>4], hexUpper[v&0x0f])
	}
}

func writeTextFields(b *buffer, fields []Field) {
	for _, field := range fields {
		if field.typ == TypeErr {
			b.buf = append(b.buf, "  \x1b[101m "...)
			b.buf = append(b.buf, field.key...)
			b.buf = append(b.buf, " \x1b[0m "...)
		} else {
			b.buf = append(b.buf, "  \x1b[100m "...)
			b.buf = append(b.buf, field.key...)
			b.buf = append(b.buf, " \x1b[0m "...)
		}
		switch field.typ {
		case TypeBool:
			if field.ival == 1 {
				b.buf = append(b.buf, "true"...)
			} else {
				b.buf = append(b.buf, "false"...)
			}
		case TypeBools:
			val := field.xval.([]bool)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				if v {
					b.buf = append(b.buf, "true"...)
				} else {
					b.buf = append(b.buf, "false"...)
				}
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeBytes:
			b.buf = append(b.buf, '"')
			writeTextBytes(b, field.xval.([]byte))
			b.buf = append(b.buf, '"')
		case TypeBytesArray:
			val := field.xval.([][]byte)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = append(b.buf, '"')
				writeTextBytes(b, v)
				b.buf = append(b.buf, '"')
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeDigest:
			writeTextDigest(b, field.xval.([]byte))
		case TypeDigests:
			val := field.xval.([][]byte)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				writeTextDigest(b, v)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeDuration:
			b.buf = append(b.buf, time.Duration(field.ival).String()...)
		case TypeDurations:
			val := field.xval.([]time.Duration)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = append(b.buf, v.String()...)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeErr:
			b.buf = append(b.buf, '"')
			writeTextString(b, field.sval)
			b.buf = append(b.buf, '"')
		case TypeErrs:
			val := field.xval.([]error)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = append(b.buf, '"')
				writeTextString(b, v.Error())
				b.buf = append(b.buf, '"')
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeFloat32:
			b.buf = strconv.AppendFloat(b.buf, float64(math.Float32frombits(uint32(field.ival))), 'f', -1, 32)
		case TypeFloat32s:
			val := field.xval.([]float32)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = strconv.AppendFloat(b.buf, float64(v), 'f', -1, 32)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeFloat64:
			b.buf = strconv.AppendFloat(b.buf, math.Float64frombits(uint64(field.ival)), 'f', -1, 64)
		case TypeFloat64s:
			val := field.xval.([]float64)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = strconv.AppendFloat(b.buf, v, 'f', -1, 64)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeInt, TypeInt8, TypeInt32, TypeInt64:
			b.buf = strconv.AppendInt(b.buf, int64(field.ival), 10)
		case TypeInts:
			val := field.xval.([]int)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = strconv.AppendInt(b.buf, int64(v), 10)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeInt8s:
			val := field.xval.([]int8)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = strconv.AppendInt(b.buf, int64(v), 10)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeInt16s:
			val := field.xval.([]int16)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = strconv.AppendInt(b.buf, int64(v), 10)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeInt32s:
			val := field.xval.([]int32)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = strconv.AppendInt(b.buf, int64(v), 10)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeInt64s:
			val := field.xval.([]int64)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = strconv.AppendInt(b.buf, v, 10)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeString:
			b.buf = append(b.buf, '"')
			writeTextString(b, field.sval)
			b.buf = append(b.buf, '"')
		case TypeStrings:
			val := field.xval.([]string)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = append(b.buf, '"')
				writeTextString(b, v)
				b.buf = append(b.buf, '"')
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeTime:
			b.buf = field.xval.(time.Time).AppendFormat(b.buf, timefmt)
		case TypeTimes:
			val := field.xval.([]time.Time)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = v.AppendFormat(b.buf, timefmt)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeUint, TypeUint8, TypeUint32, TypeUint64:
			b.buf = strconv.AppendUint(b.buf, uint64(field.ival), 10)
		case TypeUints:
			val := field.xval.([]uint)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = strconv.AppendUint(b.buf, uint64(v), 10)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeUint8s:
			val := field.xval.([]uint8)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = strconv.AppendUint(b.buf, uint64(v), 10)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeUint16s:
			val := field.xval.([]uint16)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = strconv.AppendUint(b.buf, uint64(v), 10)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeUint32s:
			val := field.xval.([]uint32)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = strconv.AppendUint(b.buf, uint64(v), 10)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		case TypeUint64s:
			val := field.xval.([]uint64)
			last := len(val) - 1
			b.buf = append(b.buf, '[')
			for i, v := range val {
				b.buf = strconv.AppendUint(b.buf, v, 10)
				if i != last {
					b.buf = append(b.buf, ',', ' ')
				}
			}
			b.buf = append(b.buf, ']')
		default:
			panic(fmt.Errorf("log: text encoding for field type not implemented: %s", field.typ))
		}
	}
}

// Adapted from the MIT-licensed zap.
func writeTextString(b *buffer, s string) {
	for i := 0; i < len(s); {
		if writeTextByte(b, s[i]) {
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.buf = append(b.buf, `\ufffd`...)
			i++
			continue
		}
		b.buf = append(b.buf, s[i:i+size]...)
		i += size
	}
}

// ToConsole sets the log level of the console logger. By default it will be
// logging at DebugLevel until this function is used to change the setting.
func ToConsole(lvl Level) {
	log.SetFlags(0)
	log.SetOutput(&intercept{})
	log.SetPrefix("")
	consoleLevel = lvl
	updateMinLevels()
}

// ToFile starts logging at the given file path.
func ToFile(path string, lvl Level) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	fileMu.Lock()
	logFile = f
	fileLevel = lvl
	fileMu.Unlock()
	updateMinLevels()
	return nil
}
