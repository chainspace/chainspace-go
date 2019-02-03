package broadcast

import (
	"sync"
)

type ownBlocks struct {
	data   map[uint64]*SignedData
	latest uint64
	mu     sync.Mutex
}

func (o *ownBlocks) get(round uint64) *SignedData {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.data[round]
}

func (o *ownBlocks) prune(size int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.data) <= size {
		return
	}
	limit := o.latest - uint64(size)
	ndata := map[uint64]*SignedData{}
	for round, block := range o.data {
		if round >= limit {
			ndata[round] = block
		}
	}
	o.data = ndata
}

func (o *ownBlocks) set(round uint64, block *SignedData) {
	o.mu.Lock()
	o.data[round] = block
	o.latest = round
	o.mu.Unlock()
}

type receivedInfo struct {
	latest   uint64
	sequence uint64
}

type receivedMap struct {
	data map[uint64]receivedInfo
	mu   sync.RWMutex
}

func (r *receivedMap) get(nodeID uint64) receivedInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.data[nodeID]
}

func (r *receivedMap) set(nodeID uint64, round uint64) {
	r.mu.Lock()
	info := r.data[nodeID]
	if info.latest < round {
		info.latest = round
	}
	if info.sequence+1 == round {
		info.sequence = round
	}
	r.data[nodeID] = info
	r.mu.Unlock()
}

func (r *receivedMap) setSequence(nodeID uint64, round uint64) {
	r.mu.Lock()
	info := r.data[nodeID]
	info.sequence = round
	r.data[nodeID] = info
	r.mu.Unlock()
}
