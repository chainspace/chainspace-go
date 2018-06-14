// Package signature provides support for digital signature algorithms.
package signature // import "chainspace.io/prototype/crypto/signature"

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/ed25519"
)

// The various digital signature schemes that are currently supported.
const (
	Ed25519 Algorithm = iota + 1
)

// Algorithm represents a digital signature algorithm.
type Algorithm uint8

func (s Algorithm) String() string {
	switch s {
	case Ed25519:
		return "ed25519"
	}
	return ""
}

// KeyPair represents the combined public/private keypair for a digital
// signature algorithm.
type KeyPair interface {
	Algorithm() Algorithm
	PrivateKey() PrivateKey
	PublicKey() PublicKey
	Sign(data []byte) (sig []byte)
	Value() []byte
	Verify(data []byte, sig []byte) bool
}

// PrivateKey represents the private key for a digital signature algorithm. It
// can be used to sign data that can be verified by holders of the corresponding
// public key.
type PrivateKey interface {
	Algorithm() Algorithm
	Sign(data []byte) (sig []byte)
	Value() []byte
}

// PublicKey represents the public key for a digital signature algorithm. It can
// be used to verify data that has been signed by the corresponding private key.
type PublicKey interface {
	Algorithm() Algorithm
	Value() []byte
	Verify(data []byte, sig []byte) bool
}

// GenKeyPair generates a new keypair for the given digital signature algorithm.
// It uses crypto/rand's Reader behind the scenes as the source of randomness.
func GenKeyPair(s Algorithm) (KeyPair, error) {
	switch s {
	case Ed25519:
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		return &ed25519KeyPair{
			priv: ed25519PrivKey(priv),
			pub:  ed25519PubKey(pub),
		}, nil
	default:
		return nil, fmt.Errorf("signature: unknown digital signature algorithm: %s", s)
	}
}

// LoadKeyPair deserialises the given value into a type representing the
// combined public/private keypair for the given digital signature algorithm.
func LoadKeyPair(s Algorithm, value []byte) (KeyPair, error) {
	switch s {
	case Ed25519:
		if len(value) != 96 {
			return nil, fmt.Errorf("signature: the length of an Ed25519 keypair should be 96 bytes, got a key with length: %d", len(value))
		}
		return ed25519KeyPair{
			priv: value[32:],
			pub:  value[:32],
		}, nil
	default:
		return nil, fmt.Errorf("signature: unknown digital signature algorithm: %s", s)
	}
}

// LoadPrivateKey deserialises the given value into a type implementing the
// PrivateKey interface for the given digital signature algorithm.
func LoadPrivateKey(s Algorithm, value []byte) (PrivateKey, error) {
	switch s {
	case Ed25519:
		if len(value) != 64 {
			return nil, fmt.Errorf("signature: the length of an Ed25519 private key should be 64 bytes, got a key with length: %d", len(value))
		}
		return ed25519PrivKey(value), nil
	default:
		return nil, fmt.Errorf("signature: unknown digital signature algorithm: %s", s)
	}
}

// LoadPublicKey deserialises the given value into a type implementing the
// PublicKey interface for the given digital signature algorithm.
func LoadPublicKey(s Algorithm, value []byte) (PublicKey, error) {
	switch s {
	case Ed25519:
		if len(value) != 32 {
			return nil, fmt.Errorf("signature: the length of an Ed25519 public key should be 32 bytes, got a key with length: %d", len(value))
		}
		return ed25519PubKey(value), nil
	default:
		return nil, fmt.Errorf("signature: unknown digital signature algorithm: %s", s)
	}
}
