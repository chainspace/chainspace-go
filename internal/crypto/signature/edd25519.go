package signature

import (
	"golang.org/x/crypto/ed25519"
)

type ed25519KeyPair struct {
	pub  []byte
	priv []byte
}

func (k ed25519KeyPair) Algorithm() Algorithm {
	return Ed25519
}

func (k ed25519KeyPair) PublicKey() PublicKey {
	return ed25519PubKey(k.pub)
}

func (k ed25519KeyPair) PrivateKey() PrivateKey {
	return ed25519PrivKey(k.priv)
}

func (k ed25519KeyPair) Sign(data []byte) []byte {
	return ed25519.Sign(ed25519.PrivateKey(k.priv), data)
}

func (k ed25519KeyPair) Value() []byte {
	v := make([]byte, 96)
	copy(v, k.pub)
	copy(v[32:], k.priv)
	return v
}

func (k ed25519KeyPair) Verify(data []byte, sig []byte) bool {
	return ed25519.Verify(ed25519.PublicKey(k.pub), data, sig)
}

type ed25519PrivKey []byte

func (k ed25519PrivKey) Algorithm() Algorithm {
	return Ed25519
}

func (k ed25519PrivKey) Sign(data []byte) []byte {
	return ed25519.Sign(ed25519.PrivateKey(k), data)
}

func (k ed25519PrivKey) Value() []byte {
	return k
}

type ed25519PubKey []byte

func (k ed25519PubKey) Algorithm() Algorithm {
	return Ed25519
}

func (k ed25519PubKey) Value() []byte {
	return k
}

func (k ed25519PubKey) Verify(data []byte, sig []byte) bool {
	return ed25519.Verify(ed25519.PublicKey(k), data, sig)
}

var _ KeyPair = (*ed25519KeyPair)(nil)
var _ PrivateKey = (*ed25519PrivKey)(nil)
var _ PublicKey = (*ed25519PubKey)(nil)
