package broadcast

import (
	"chainspace.io/prototype/combihash"
)

// Size returns the rough size of the entry in bytes.
func (e *Entry) Size() int {
	switch e := e.Value.(type) {
	case *Entry_Block:
		return len(e.Block.Hash) + 100
	case *Entry_Transaction:
		return len(e.Transaction.Data) + 100
	default:
		panic("broadcast: unknown Entry type")
	}
}

// Digest returns the combihash of the signed block.
func (s *SignedBlock) Digest() []byte {
	h := combihash.New()
	_, err := h.Write(s.Data)
	if err != nil {
		panic(err)
	}
	return h.Digest()
}
