package lexinum

import (
	"bytes"
	"testing"
)

func TestDecode(t *testing.T) {
	for i := uint64(0); i < 8258175*5; i++ {
		d := Encode(i)
		v, err := Decode(d)
		if err != nil {
			t.Fatalf("could not decode the encoding of %d: %s", i, err)
		}
		if v != i {
			t.Fatalf("decoding did not match encoding input: got %d, expected %d; encoded: %q", v, i, d)
		}
	}
}

func TestEncode(t *testing.T) {
	var prev []byte
	for i := uint64(0); i < 8258175*5; i++ {
		cur := Encode(i)
		if bytes.Compare(cur, prev) != 1 {
			t.Fatalf("encoding of %d does not lexicographically follow from %d", i, i-1)
		}
		prev = cur
	}
}
