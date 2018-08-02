package byzco

import (
	"context"
	"sync"
	"testing"
)

func TestBasic(t *testing.T) {}

func TestGraph(t *testing.T) {
	var (
		lastRound uint64
		mu        sync.Mutex
	)
	cb := func(i *Interpreted) {
		mu.Lock()
		defer mu.Unlock()
		if i.Round == 1 {
			if len(i.Blocks) != 4 {
				t.Fatalf("results for round 1 are of the wrong length: got %d, expected %d", len(i.Blocks), 4)
			}
		}
	}
	blocks := map[BlockID][]BlockID{}
	g := New(context.Background(), []uint64{1, 2, 3, 4}, 0, cb)
	for from, to := range blocks {
		g.Add(from, to)
	}
	mu.Lock()
	defer mu.Unlock()
	if lastRound != 0 {
		t.Fatalf("last round did not match: got %d, expected %d", lastRound, 0)
	}
}
