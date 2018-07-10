package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Constants representing various SI multiples for bytes.
const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
	TB = 1024 * GB
)

const maxInt = ByteSize(^uint(0) >> 1)

// ByteSize provides a YAML-serializable format for byte size definitions.
type ByteSize uint64

// Int returns the ByteSize value as an int as long as it'd fit within the
// system's int limits.
func (b ByteSize) Int() (int, error) {
	if b > maxInt {
		return 0, fmt.Errorf("config: ByteSize value %d overflows platform int", b)
	}
	return int(b), nil
}

// MarshalYAML implements the YAML encoding interface.
func (b ByteSize) MarshalYAML() (interface{}, error) {
	switch {
	// case b == 0:
	// 	return "", nil
	case b%TB == 0:
		return strconv.FormatUint(uint64(b)/TB, 10) + "TB", nil
	case b%GB == 0:
		return strconv.FormatUint(uint64(b)/GB, 10) + "GB", nil
	case b%MB == 0:
		return strconv.FormatUint(uint64(b)/MB, 10) + "MB", nil
	case b%KB == 0:
		return strconv.FormatUint(uint64(b)/KB, 10) + "KB", nil
	default:
		return strconv.FormatUint(uint64(b), 10) + "B", nil
	}
}

// UnmarshalYAML implements the YAML decoding interface.
func (b *ByteSize) UnmarshalYAML(unmarshal func(interface{}) error) error {
	raw := ""
	if err := unmarshal(&raw); err != nil {
		return err
	}
	// TODO(tav): Insert overflow checks.
	suffix := ""
	for i := len(raw) - 1; i >= 0; i-- {
		char := raw[i]
		if char >= 48 && char <= 57 {
			val, err := strconv.ParseUint(raw[:i+1], 10, 64)
			if err != nil {
				return err
			}
			*b = ByteSize(val)
			break
		}
		suffix = string(char) + suffix
	}
	suffix = strings.ToLower(suffix)
	switch suffix {
	case "":
		return nil
	case "b", "byte", "bytes":
		return nil
	case "k", "kb":
		*b = *b * KB
		return nil
	case "m", "mb":
		*b = *b * MB
		return nil
	case "g", "gb":
		*b = *b * GB
		return nil
	case "t", "tb":
		*b = *b * TB
		return nil
	default:
		return fmt.Errorf("config: unable to decode ByteSize value: %q", raw)
	}
}
