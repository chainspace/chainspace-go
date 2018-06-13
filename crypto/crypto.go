package crypto

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/ed25519"
)

type KeyPair struct {
	PrivKey
	PubKey
}

type PrivKey interface {
	PrivateKey() []byte
	Sign(data []byte) (sig []byte)
}

type PubKey interface {
	Algorithm() string
	PublicKey() []byte
	Verify(data []byte, sig []byte) bool
}

func GenKeyPair(alg string) (*KeyPair, error) {
	switch alg {
	case "ed25519":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		return &KeyPair{
			PrivKey: ed25519Privkey(priv),
			PubKey:  ed25519Pubkey(pub),
		}, nil
	default:
		return nil, fmt.Errorf("crypto: unknown key generation algorithm: %s", alg)
	}
}
