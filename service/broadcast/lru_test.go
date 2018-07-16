package broadcast

import (
	"testing"
)

func TestLRU(t *testing.T) {
	l := &lru{
		data: map[blockPointer]*blockInfoContainer{},
	}
	keys := []blockPointer{
		{1, 1, ""},
		{1, 2, ""},
		{2, 2, ""},
		{2, 3, ""},
		{2, 4, ""},
	}
	for _, key := range keys {
		l.set(key, &blockInfo{})
	}
	info := l.get(blockPointer{1, 2, ""})
	if info == nil {
		t.Fatal("expected blockinfo for {1, 2}, got nil")
	}
	info = l.get(blockPointer{2, 2, ""})
	if info == nil {
		t.Fatal("expected blockinfo for {2, 2}, got nil")
	}
	info = l.get(blockPointer{1, 1, ""})
	if info == nil {
		t.Fatal("expected blockinfo for {1, 1}, got nil")
	}
	info = l.get(blockPointer{3, 1, ""})
	if info != nil {
		t.Fatal("expected nil blockinfo for {3, 1}, got value")
	}
	l.prune(2)
	if len(l.data) != 2 {
		t.Fatalf("expected size of LRU to be 2, got %d", len(l.data))
	}
	info = l.get(blockPointer{1, 2, ""})
	if info != nil {
		t.Fatal("expected nil blockinfo for {1, 2}, got value")
	}
	info = l.get(blockPointer{2, 2, ""})
	if info == nil {
		t.Fatal("expected blockinfo for {2, 2}, got nil")
	}
	info = l.get(blockPointer{1, 1, ""})
	if info == nil {
		t.Fatal("expected blockinfo for {1, 1}, got nil")
	}
}
