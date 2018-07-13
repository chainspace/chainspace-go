package broadcast

import (
	"sort"
	"sync"
)

type blockInfo struct {
	block *SignedData
	hash  []byte
}

type blockInfoContainer struct {
	info     *blockInfo
	lastSeen uint64
}

type blockPointer struct {
	node  uint64
	round uint64
}

type lru struct {
	data   map[blockPointer]*blockInfoContainer
	mu     sync.Mutex
	seenID uint64
}

func (l *lru) get(key blockPointer) *blockInfo {
	l.mu.Lock()
	c, exists := l.data[key]
	if !exists {
		l.mu.Unlock()
		return nil
	}
	l.seenID++
	c.lastSeen = l.seenID
	l.mu.Unlock()
	return c.info
}

func (l *lru) set(key blockPointer, value *blockInfo) {
	l.mu.Lock()
	l.seenID++
	c := &blockInfoContainer{value, l.seenID}
	l.data[key] = c
	l.mu.Unlock()
}

func (l *lru) evict(limit int) {
	type kv struct {
		key   blockPointer
		value *blockInfoContainer
	}
	var xs []kv
	l.mu.Lock()
	for k, v := range l.data {
		xs = append(xs, kv{k, v})
	}
	sort.Slice(xs, func(i, j int) bool {
		return xs[i].value.lastSeen < xs[j].value.lastSeen
	})
	xs = xs[:limit]
	data := map[blockPointer]*blockInfoContainer{}
	for _, kv := range xs {
		data[kv.key] = kv.value
	}
	l.data = data
	l.mu.Unlock()
}
