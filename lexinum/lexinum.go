// Package lexinum supports encoding of uint64s in a lexicographically sortable
// manner.
package lexinum // import "chainspace.io/prototype/lexinum"

import (
	"fmt"
)

const maxSmall = 8258175

func pow(a, b uint64) uint64 {
	n := uint64(1)
	for b > 0 {
		if b&1 != 0 {
			n *= a
		}
		b >>= 1
		a *= a
	}
	return n
}

// Decode returns the deserialised uint64 value, along with the slice index of
// the read data.
func Decode(d []byte) (uint64, error) {
	if len(d) < 2 {
		return 0, fmt.Errorf("lexinum: received too small a slice to decode")
	}
	if d[0] < 0x80 {
		return 0, fmt.Errorf("lexinum: only support for positive numbers have been implemented: received first byte %q", d[0])
	}
	// Handle big positive integers.
	if d[0] == 0xff {
		var (
			rem uint64
			v   uint64
		)
		for i, char := range d[2:] {
			if char == 0x01 {
				// TODO(tav): Maybe check for out of bounds error here.
				rem = uint64(d[i+3])
				break
			}
			v += (v * 252) + uint64(char-2)
		}
		return (v * 255) + rem + maxSmall, nil
	}
	// Handle small positive integers.
	if len(d) < 3 {
		return 0, fmt.Errorf("lexinum: received too small a slice to decode a small positive integer")
	}
	return (((uint64(d[0]-128) * 255) + uint64(d[1]-1)) * 255) + uint64(d[2]-1), nil
}

// Encode serialises the given uint64 so that the encoded form is
// lexicographically comparable in numeric order.
func Encode(v uint64) []byte {
	if v == 0 {
		return []byte{0x80, 0x01, 0x01}
	}
	// Handle small positive integers.
	if v < maxSmall {
		enc := []byte{0x80, 0x01, 0x01}
		div, mod := v/255, v%255
		enc[2] = byte(mod + 1)
		if div > 0 {
			div, mod = div/255, div%255
			enc[1] = byte(mod + 1)
			if div > 0 {
				enc[0] = byte(div + 128)
			}
		}
		return enc
	}
	// Handle big positive integers.
	v -= maxSmall
	enc := []byte{0xff, 0xff}
	lead, rem := v/255, v%255
	n := uint64(1)
	for lead/pow(253, n) > 0 {
		n++
	}
	enc[1] = byte(n) + 1
	var (
		chars []byte
		mod   uint64
	)
	for {
		if lead == 0 {
			break
		}
		lead, mod = lead/253, lead%253
		chars = append(chars, byte(mod+2))
	}
	for i := len(chars) - 1; i >= 0; i-- {
		enc = append(enc, chars[i])
	}
	if rem > 0 {
		enc = append(enc, 0x01, byte(rem))
	}
	return enc
}
