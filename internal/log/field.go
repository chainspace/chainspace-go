package log // import "chainspace.io/prototype/internal/log"

import (
	"fmt"
	"math"
	"time"
)

// Log field types.
const (
	TypeBool fieldType = iota + 1
	TypeBools
	TypeBytes
	TypeBytesArray
	TypeDigest
	TypeDigests
	TypeDuration
	TypeDurations
	TypeErr
	TypeErrs
	TypeFloat32
	TypeFloat32s
	TypeFloat64
	TypeFloat64s
	TypeInt
	TypeInts
	TypeInt8
	TypeInt8s
	TypeInt16
	TypeInt16s
	TypeInt32
	TypeInt32s
	TypeInt64
	TypeInt64s
	TypeString
	TypeStrings
	TypeTime
	TypeTimes
	TypeUint
	TypeUints
	TypeUint8
	TypeUint8s
	TypeUint16
	TypeUint16s
	TypeUint32
	TypeUint32s
	TypeUint64
	TypeUint64s
)

type fieldType byte

func (f fieldType) String() string {
	switch f {
	case TypeBool:
		return "Bool"
	case TypeBools:
		return "Bools"
	case TypeBytes:
		return "Bytes"
	case TypeBytesArray:
		return "BytesArray"
	case TypeDigest:
		return "Digest"
	case TypeDigests:
		return "Digests"
	case TypeDuration:
		return "Duration"
	case TypeDurations:
		return "Durations"
	case TypeErr:
		return "Err"
	case TypeErrs:
		return "Errs"
	case TypeFloat32:
		return "Float32"
	case TypeFloat32s:
		return "Float32s"
	case TypeFloat64:
		return "Float64"
	case TypeFloat64s:
		return "Float64s"
	case TypeInt:
		return "Int"
	case TypeInts:
		return "Ints"
	case TypeInt8:
		return "Int8"
	case TypeInt8s:
		return "Int8s"
	case TypeInt16:
		return "Int16"
	case TypeInt16s:
		return "Int16s"
	case TypeInt32:
		return "Int32"
	case TypeInt32s:
		return "Int32s"
	case TypeInt64:
		return "Int64"
	case TypeInt64s:
		return "Int64s"
	case TypeString:
		return "String"
	case TypeStrings:
		return "Strings"
	case TypeTime:
		return "Time"
	case TypeTimes:
		return "Times"
	case TypeUint:
		return "Uint"
	case TypeUints:
		return "Uints"
	case TypeUint8:
		return "Uint8"
	case TypeUint8s:
		return "Uint8s"
	case TypeUint16:
		return "Uint16"
	case TypeUint16s:
		return "Uint16s"
	case TypeUint32:
		return "Uint32"
	case TypeUint32s:
		return "Uint32s"
	case TypeUint64:
		return "Uint64"
	case TypeUint64s:
		return "Uint64s"
	default:
		panic(fmt.Errorf("log: String method unimplemented for log type %d", f))
	}
}

// Field represents a key/value pair for adding to a log entry.
type Field struct {
	key  string
	typ  fieldType
	ival int64
	sval string
	xval interface{}
}

// Bool represents a field with a boolean value.
func Bool(key string, value bool) Field {
	if value {
		return Field{
			key:  key,
			typ:  TypeBool,
			ival: 1,
		}
	}
	return Field{
		key:  key,
		typ:  TypeBool,
		ival: 0,
	}
}

// Bools represents a field with a slice of boolean values.
func Bools(key string, value []bool) Field {
	return Field{
		key:  key,
		typ:  TypeBools,
		xval: value,
	}
}

// Bytes represents a field with a byte slice value.
func Bytes(key string, value []byte) Field {
	return Field{
		key:  key,
		typ:  TypeBytes,
		xval: value,
	}
}

// BytesArray represents a field with an array of byte slice values.
func BytesArray(key string, value [][]byte) Field {
	return Field{
		key:  key,
		typ:  TypeBytesArray,
		xval: value,
	}
}

// Digest represents a field with a hash digest value.
func Digest(key string, value []byte) Field {
	return Field{
		key:  key,
		typ:  TypeDigest,
		xval: value,
	}
}

// Digests represents a field with an array of hash digest values.
func Digests(key string, value [][]byte) Field {
	return Field{
		key:  key,
		typ:  TypeDigests,
		xval: value,
	}
}

// Duration represents a field with a duration value.
func Duration(key string, value time.Duration) Field {
	return Field{
		key:  key,
		typ:  TypeDuration,
		ival: int64(value),
	}
}

// Durations represents a field with an array of duration values.
func Durations(key string, value []time.Duration) Field {
	return Field{
		key:  key,
		typ:  TypeDurations,
		xval: value,
	}
}

// Err represents a field with an error value. It automatically uses "error" as
// the Field key.
func Err(value error) Field {
	return Field{
		key:  "error",
		typ:  TypeErr,
		sval: value.Error(),
	}
}

// Errs represents a field with an array of error values. It automatically uses
// "error" as the Field key.
func Errs(value error) Field {
	return Field{
		key:  "error",
		typ:  TypeErrs,
		xval: value,
	}
}

// Float32 represents a field with a float32 value.
func Float32(key string, value float32) Field {
	return Field{
		key:  key,
		typ:  TypeFloat32,
		ival: int64(math.Float32bits(value)),
	}
}

// Float32s represents a field with an array of float32 values.
func Float32s(key string, value []float32) Field {
	return Field{
		key:  key,
		typ:  TypeFloat32s,
		xval: value,
	}
}

// Float64 represents a field with a float64 value.
func Float64(key string, value float64) Field {
	return Field{
		key:  key,
		typ:  TypeFloat64,
		ival: int64(math.Float64bits(value)),
	}
}

// Float64s represents a field with an array of float64 values.
func Float64s(key string, value []float64) Field {
	return Field{
		key:  key,
		typ:  TypeFloat64s,
		xval: value,
	}
}

// Int represents a field with an int value.
func Int(key string, value int) Field {
	return Field{
		key:  key,
		typ:  TypeInt,
		ival: int64(value),
	}
}

// Ints represents a field with an array of int values.
func Ints(key string, value []int) Field {
	return Field{
		key:  key,
		typ:  TypeInts,
		xval: value,
	}
}

// Int8 represents a field with an int8 value.
func Int8(key string, value int8) Field {
	return Field{
		key:  key,
		typ:  TypeInt8,
		ival: int64(value),
	}
}

// Int8s represents a field with an array of int8 values.
func Int8s(key string, value []int8) Field {
	return Field{
		key:  key,
		typ:  TypeInt8s,
		xval: value,
	}
}

// Int16 represents a field with an int16 value.
func Int16(key string, value int16) Field {
	return Field{
		key:  key,
		typ:  TypeInt16,
		ival: int64(value),
	}
}

// Int16s represents a field with an array of int16 values.
func Int16s(key string, value []int16) Field {
	return Field{
		key:  key,
		typ:  TypeInt16s,
		xval: value,
	}
}

// Int32 represents a field with an int32 value.
func Int32(key string, value int32) Field {
	return Field{
		key:  key,
		typ:  TypeInt32,
		ival: int64(value),
	}
}

// Int32s represents a field with an array of int32 values.
func Int32s(key string, value []int32) Field {
	return Field{
		key:  key,
		typ:  TypeInt32s,
		xval: value,
	}
}

// Int64 represents a field with an int64 value.
func Int64(key string, value int64) Field {
	return Field{
		key:  key,
		typ:  TypeInt64,
		ival: int64(value),
	}
}

// Int64s represents a field with an array of int64 values.
func Int64s(key string, value []int64) Field {
	return Field{
		key:  key,
		typ:  TypeInt64s,
		xval: value,
	}
}

// String represents a field with a string value.
func String(key string, value string) Field {
	return Field{
		key:  key,
		typ:  TypeString,
		sval: value,
	}
}

// Strings represents a field with an array of string values.
func Strings(key string, value []string) Field {
	return Field{
		key:  key,
		typ:  TypeStrings,
		xval: value,
	}
}

// Time represents a field with a time value.
func Time(key string, value time.Time) Field {
	return Field{
		key:  key,
		typ:  TypeTime,
		xval: value,
	}
}

// Times represents a field with an array of time values.
func Times(key string, value []time.Time) Field {
	return Field{
		key:  key,
		typ:  TypeTimes,
		xval: value,
	}
}

// Uint represents a field with a uint value.
func Uint(key string, value uint) Field {
	return Field{
		key:  key,
		typ:  TypeUint,
		ival: int64(value),
	}
}

// Uints represents a field with an array of uint values.
func Uints(key string, value []uint) Field {
	return Field{
		key:  key,
		typ:  TypeUints,
		xval: value,
	}
}

// Uint8 represents a field with a uint8 value.
func Uint8(key string, value uint8) Field {
	return Field{
		key:  key,
		typ:  TypeUint8,
		ival: int64(value),
	}
}

// Uint8s represents a field with an array of uint8 values.
func Uint8s(key string, value []uint8) Field {
	return Field{
		key:  key,
		typ:  TypeUint8s,
		xval: value,
	}
}

// Uint16 represents a field with a uint16 value.
func Uint16(key string, value uint16) Field {
	return Field{
		key:  key,
		typ:  TypeUint16,
		ival: int64(value),
	}
}

// Uint16s represents a field with an array of uint16 values.
func Uint16s(key string, value []uint16) Field {
	return Field{
		key:  key,
		typ:  TypeUint16s,
		xval: value,
	}
}

// Uint32 represents a field with a uint32 value.
func Uint32(key string, value uint32) Field {
	return Field{
		key:  key,
		typ:  TypeUint32,
		ival: int64(value),
	}
}

// Uint32s represents a field with an array of uint32 values.
func Uint32s(key string, value []uint32) Field {
	return Field{
		key:  key,
		typ:  TypeUint32s,
		xval: value,
	}
}

// Uint64 represents a field with a uint64 value.
func Uint64(key string, value uint64) Field {
	return Field{
		key:  key,
		typ:  TypeUint64,
		ival: int64(value),
	}
}

// Uint64s represents a field with an array of uint64 values.
func Uint64s(key string, value []uint64) Field {
	return Field{
		key:  key,
		typ:  TypeUint64s,
		xval: value,
	}
}
