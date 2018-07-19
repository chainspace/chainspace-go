package broadcast

import (
	"testing"

	"chainspace.io/prototype/byzco"
)

func TestLRU(t *testing.T) {
	l := &lru{
		data: map[byzco.BlockID]*blockInfoContainer{},
	}
	keys := []byzco.BlockID{
		{"", 1, 1},
		{"", 1, 2},
		{"", 2, 2},
		{"", 2, 3},
		{"", 2, 4},
	}
	for _, key := range keys {
		l.set(key, &blockInfo{})
	}
	info := l.get(byzco.BlockID{"", 1, 2})
	if info == nil {
		t.Fatal("expected blockinfo for {1, 2}, got nil")
	}
	info = l.get(byzco.BlockID{"", 2, 2})
	if info == nil {
		t.Fatal("expected blockinfo for {2, 2}, got nil")
	}
	info = l.get(byzco.BlockID{"", 1, 1})
	if info == nil {
		t.Fatal("expected blockinfo for {1, 1}, got nil")
	}
	info = l.get(byzco.BlockID{"", 3, 1})
	if info != nil {
		t.Fatal("expected nil blockinfo for {3, 1}, got value")
	}
	l.prune(2)
	if len(l.data) != 2 {
		t.Fatalf("expected size of LRU to be 2, got %d", len(l.data))
	}
	info = l.get(byzco.BlockID{"", 1, 2})
	if info != nil {
		t.Fatal("expected nil blockinfo for {1, 2}, got value")
	}
	info = l.get(byzco.BlockID{"", 2, 2})
	if info == nil {
		t.Fatal("expected blockinfo for {2, 2}, got nil")
	}
	info = l.get(byzco.BlockID{"", 1, 1})
	if info == nil {
		t.Fatal("expected blockinfo for {1, 1}, got nil")
	}
}
