package broadcast

import (
	"sort"
	"sync"

	"chainspace.io/prototype/byzco"
)

type lru struct {
	data   map[byzco.BlockID]*lruData
	mu     sync.Mutex
	seenID uint64
}

func (l *lru) get(key byzco.BlockID) *SignedData {
	l.mu.Lock()
	c, exists := l.data[key]
	if !exists {
		l.mu.Unlock()
		return nil
	}
	l.seenID++
	c.lastSeen = l.seenID
	l.mu.Unlock()
	return c.block
}

func (l *lru) set(key byzco.BlockID, value *SignedData) {
	l.mu.Lock()
	l.seenID++
	c := &lruData{value, l.seenID}
	l.data[key] = c
	l.mu.Unlock()
}

func (l *lru) prune(size int) {
	type kv struct {
		key   byzco.BlockID
		value *lruData
	}
	var xs []kv
	l.mu.Lock()
	if len(l.data) <= size {
		l.mu.Unlock()
		return
	}
	for k, v := range l.data {
		xs = append(xs, kv{k, v})
	}
	sort.Slice(xs, func(i, j int) bool {
		return xs[i].value.lastSeen > xs[j].value.lastSeen
	})
	xs = xs[:size]
	data := map[byzco.BlockID]*lruData{}
	for _, kv := range xs {
		data[kv.key] = kv.value
	}
	l.data = data
	l.mu.Unlock()
}

type lruData struct {
	block    *SignedData
	lastSeen uint64
}
