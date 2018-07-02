package broadcast

import (
	"chainspace.io/prototype/combihash"
)

// Digest returns the combihash of the signed block.
func (s *SignedBlock) Digest() []byte {
	h := combihash.New()
	_, err := h.Write(s.Data)
	if err != nil {
		panic(err)
	}
	return h.Digest()
}
