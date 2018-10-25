// Package combihash implements the combined SHAKE256 + SHA-512/256 hash format.
//
// The generated digest is of the form:
//
//     <version-prefix> <data-length> <shake256-digest> <sha-512/256-digest>
//     <2 bytes>        <4 bytes>     <64 bytes>        <32 bytes>
//
// The version prefix is `v1` and the data length is encoded as an unsigned
// 32-bit in little endian format.
package combihash // import "chainspace.io/prototype/internal/combihash"

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash"

	"golang.org/x/crypto/sha3"
)

// Size of the combihash digest.
const Size = 102

// Error values.
var (
	ErrOverflow    = errors.New("combihash: number of written bytes exceeds uint32")
	ErrAlreadyRead = errors.New("combihash: cannot write to hash that has been read from")
)

// State represents the internal combihash state.
type State struct {
	read    bool
	sha512  hash.Hash
	shake   sha3.ShakeHash
	written uint32
}

// Base64 returns the digest in base64-encoded format.
func (s *State) Base64() []byte {
	buf := make([]byte, 136)
	base64.RawURLEncoding.Encode(buf, s.Digest())
	return buf
}

// Digest returns the combihash for the data written.
func (s *State) Digest() []byte {
	s.read = true
	buf := make([]byte, 70)
	buf[0] = 'v'
	buf[1] = '1'
	binary.LittleEndian.PutUint32(buf[2:6], s.written)
	s.shake.Read(buf[6:70])
	return s.sha512.Sum(buf)
}

// Reset resets the combihasher to its initial state.
func (s *State) Reset() {
	s.read = false
	s.written = 0
	s.sha512.Reset()
	s.shake.Reset()
}

// Write updates the underlying state of the combihash. It is illegal to call
// Write after reading the digest value, as that changes the underlying state in
// SHAKE256.
func (s *State) Write(p []byte) (int, error) {
	if s.read {
		return 0, ErrAlreadyRead
	}
	size := len(p)
	if size > 4294967295 {
		return 0, ErrOverflow
	}
	if (s.written + uint32(size)) < s.written {
		return 0, ErrOverflow
	}
	n, err := s.sha512.Write(p)
	if err != nil {
		return n, err
	}
	n, err = s.shake.Write(p)
	if err != nil {
		return n, err
	}
	return size, nil
}

// New returns a combihasher.
func New() *State {
	return &State{
		sha512: sha512.New512_256(),
		shake:  sha3.NewShake256(),
	}
}
