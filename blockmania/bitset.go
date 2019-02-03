package blockmania

import (
	"math/bits"
)

type bitset struct {
	commits  []uint64
	prepares []uint64
}

func (b *bitset) clone() *bitset {
	n := &bitset{
		commits:  make([]uint64, len(b.commits)),
		prepares: make([]uint64, len(b.prepares)),
	}
	copy(n.commits, b.commits)
	copy(n.prepares, b.prepares)
	return n
}

func (b *bitset) commitCount() int {
	c := 0
	for _, word := range b.commits {
		c += bits.OnesCount64(word)
	}
	return c
}

func (b *bitset) hasCommit(v uint64) bool {
	return b.commits[int(v>>6)]&(1<<(v&63)) != 0
}

func (b *bitset) hasPrepare(v uint64) bool {
	return b.prepares[int(v>>6)]&(1<<(v&63)) != 0
}

func (b *bitset) prepareCount() int {
	c := 0
	for _, word := range b.prepares {
		c += bits.OnesCount64(word)
	}
	return c
}

func (b *bitset) setCommit(v uint64) {
	b.commits[int(v>>6)] |= 1 << (v & 63)
}

func (b *bitset) setPrepare(v uint64) {
	b.prepares[int(v>>6)] |= 1 << (v & 63)
}

func newBitset(size int) *bitset {
	words := (size + 63) >> 6
	return &bitset{
		commits:  make([]uint64, words),
		prepares: make([]uint64, words),
	}
}
