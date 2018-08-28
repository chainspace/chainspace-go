package broadcast

import (
	"bytes"
	"crypto/sha512"
	"encoding/binary"
	"sort"

	"chainspace.io/prototype/log"
	"github.com/gogo/protobuf/proto"
)

// TransactionData represents the transaction's combined data and fee.
type TransactionData struct {
	Data []byte
	Fee  uint64
}

// TransactionList wraps a slice of TransactionData for the purposes of sorting.
type TransactionList []TransactionData

// Sort orders the TransactionList in descending order of the transaction fee
// and then hash if the fees are equal.
func (t TransactionList) Sort() {
	hasher := sha512.New512_256()
	sort.Slice(t, func(i, j int) bool {
		if t[i].Fee == t[j].Fee {
			if _, err := hasher.Write(t[i].Data); err != nil {
				log.Fatal("Couldn't hash transaction data")
			}
			ihash := hasher.Sum(nil)
			hasher.Reset()
			if _, err := hasher.Write(t[j].Data); err != nil {
				log.Fatal("Couldn't hash transaction data")
			}
			jhash := hasher.Sum(nil)
			hasher.Reset()
			return bytes.Compare(ihash, jhash) == 1
		}
		return t[i].Fee > t[j].Fee
	})
}

// TxIterator provides support for iterating over a Block's transactions.
type TxIterator struct {
	cur    uint64
	data   []byte
	idx    int
	total  uint64
	TxData []byte
	TxFee  uint64
}

func (t *TxIterator) next() {
	size := int(binary.LittleEndian.Uint32(t.data[t.idx : t.idx+4]))
	t.idx += 4
	t.TxData = t.data[t.idx : t.idx+size]
	t.idx += size
	t.TxFee = binary.LittleEndian.Uint64(t.data[t.idx : t.idx+8])
	t.idx += 8
	t.cur++
}

// Next moves the iterator forward. The TxData and TxFee values of the iterator
// are only valid until the next call to Next().
func (t *TxIterator) Next() {
	t.next()
}

// Valid returns whether the iterator is still valid and has more elements to
// iterate over. On first call, it also moves the iterator forward, so as to
// simplify the for loop used by consumers.
func (t *TxIterator) Valid() bool {
	if t.cur == 0 {
		if t.cur >= t.total {
			return false
		}
		t.next()
		return true
	}
	return t.cur < t.total
}

// Iter returns a iterator for the block's transactions.
func (b *Block) Iter() *TxIterator {
	return &TxIterator{
		cur:   0,
		data:  b.Transactions.Data,
		total: b.Transactions.Count,
	}
}

// Size returns the rough size of a BlockReference in bytes.
func (b *BlockReference) Size() int {
	return len(b.Hash) + 100
}

// Block returns the decoded Block struct assuming the SignedData is of a Block.
func (s *SignedData) Block() (*Block, error) {
	block := &Block{}
	if err := proto.Unmarshal(s.Data, block); err != nil {
		return nil, err
	}
	return block, nil
}

// Digest returns the hash of the underlying data.
func (s *SignedData) Digest() []byte {
	hasher := sha512.New512_256()
	if _, err := hasher.Write(s.Data); err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

// Size returns the rough size of the SignedData in bytes.
func (s *SignedData) Size() int {
	return len(s.Data) + len(s.Signature) + 100
}
