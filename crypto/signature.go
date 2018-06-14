package crypto

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/ed25519"
)

// The various digital signature schemes that are currently supported.
const (
	Ed25519Signature SignatureAlgorithm = iota + 1
)

// SignatureAlgorithm represents a digital signature algorithm.
type SignatureAlgorithm int8

func (s SignatureAlgorithm) String() string {
	switch s {
	case Ed25519Signature:
		return "ed25519"
	}
	return ""
}

type SignatureKeyPair struct {
	SignaturePrivKey
	SignaturePubKey
}

type SignaturePrivKey interface {
	PrivateKey() []byte
	Sign(data []byte) (sig []byte)
}

type SignaturePubKey interface {
	Algorithm() SignatureAlgorithm
	PublicKey() []byte
	Verify(data []byte, sig []byte) bool
}

func GenSignatureKey(s SignatureAlgorithm) (*SignatureKeyPair, error) {
	switch s {
	case Ed25519Signature:
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		return &SignatureKeyPair{
			SignaturePrivKey: ed25519Privkey(priv),
			SignaturePubKey:  ed25519Pubkey(pub),
		}, nil
	default:
		return nil, fmt.Errorf("crypto: unknown digital signature scheme: %s", s)
	}
}
