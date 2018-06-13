package crypto

import (
	"golang.org/x/crypto/ed25519"
)

type ed25519Privkey []byte

func (k ed25519Privkey) PrivateKey() []byte {
	return k
}

func (k ed25519Privkey) Sign(data []byte) []byte {
	return ed25519.Sign(ed25519.PrivateKey(k), data)
}

type ed25519Pubkey []byte

func (k ed25519Pubkey) Algorithm() string {
	return "ed25519"
}

func (k ed25519Pubkey) PublicKey() []byte {
	return k
}

func (k ed25519Pubkey) Verify(data []byte, sig []byte) bool {
	return ed25519.Verify(ed25519.PublicKey(k), data, sig)
}
