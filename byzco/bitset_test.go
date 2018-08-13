package byzco

import (
	"testing"
)

func TestBitset(t *testing.T) {
	b := newBitset(12)
	b.setCommit(4)
	b.setCommit(7)
	b.setCommit(10)
	if b.commitCount() != 3 {
		t.Fatalf("wrong commit count: expected 3, got %d", b.commitCount())
	}
	b2 := b.clone()
	b.setCommit(1)
	b2.setCommit(4)
	if b.commitCount() != 4 {
		t.Fatalf("wrong commit count: expected 4, got %d", b.commitCount())
	}
	if b2.commitCount() != 3 {
		t.Fatalf("wrong commit count: expected 3, got %d", b2.commitCount())
	}
}
