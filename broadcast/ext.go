package broadcast

import (
	"bytes"
	"sort"

	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/log"
	"github.com/gogo/protobuf/proto"
)

// transactionList wraps a slice of TransactionData for the purposes of sorting.
type transactionList []*TransactionData

// Sort orders the transactionList in descending order of the transaction fee
// and then hash if the fees are equal.
func (t transactionList) Sort() {
	hasher := combihash.New()
	sort.Slice(t, func(i, j int) bool {
		if t[i].Fee == t[j].Fee {
			if _, err := hasher.Write(t[i].Data); err != nil {
				log.Fatal("Couldn't hash transaction data")
			}
			ihash := hasher.Digest()
			hasher.Reset()
			if _, err := hasher.Write(t[j].Data); err != nil {
				log.Fatal("Couldn't hash transaction data")
			}
			jhash := hasher.Digest()
			hasher.Reset()
			return bytes.Compare(ihash, jhash) == 1
		}
		return t[i].Fee > t[j].Fee
	})
}

// Size returns the rough size of a BlockReference in bytes.
func (b *BlockReference) Size() int {
	return len(b.Hash) + 100
}

// Digest returns the combihash of the underlying data.
func (s *SignedData) Digest() []byte {
	hasher := combihash.New()
	if _, err := hasher.Write(s.Data); err != nil {
		panic(err)
	}
	return hasher.Digest()
}

// Size returns the rough size of the SignedData in bytes.
func (s *SignedData) Size() int {
	return len(s.Data) + len(s.Signature) + 100
}

// Transactions returns the block transactions assuming the SignedData is of a
// Block.
func (s *SignedData) Transactions() ([]*TransactionData, error) {
	block := &Block{}
	if err := proto.Unmarshal(s.Data, block); err != nil {
		return nil, err
	}
	return block.Transactions, nil
}

// Size returns the rough size of the TransactionData in bytes.
func (t *TransactionData) Size() int {
	return len(t.Data) + 100
}
