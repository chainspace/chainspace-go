package byzco

import (
	"math/bits"
)

type bitset struct {
	cms []uint64
	prs []uint64
}

func (b *bitset) clone() *bitset {
	n := &bitset{
		cms: make([]uint64, len(b.cms)),
		prs: make([]uint64, len(b.prs)),
	}
	copy(n.cms, b.cms)
	copy(n.prs, b.prs)
	return n
}

func (b *bitset) commitCount() int {
	c := 0
	for _, word := range b.cms {
		c += bits.OnesCount64(word)
	}
	return c
}

func (b *bitset) hasCommit(v uint64) bool {
	return b.cms[int(v>>6)]&(1<<(v&63)) != 0
}

func (b *bitset) hasPrepare(v uint64) bool {
	return b.prs[int(v>>6)]&(1<<(v&63)) != 0
}

func (b *bitset) prepareCount() int {
	c := 0
	for _, word := range b.prs {
		c += bits.OnesCount64(word)
	}
	return c
}

func (b *bitset) setCommit(v uint64) {
	b.cms[int(v>>6)] |= 1 << (v & 63)
}

func (b *bitset) setPrepare(v uint64) {
	b.prs[int(v>>6)] |= 1 << (v & 63)
}

func newBitset(size int) *bitset {
	words := (size + 63) >> 6
	return &bitset{
		cms: make([]uint64, words),
		prs: make([]uint64, words),
	}
}
