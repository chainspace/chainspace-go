package broadcast

import (
	"context"
	"sync"

	"chainspace.io/prototype/byzco"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
)

type blockData struct {
	deps []byzco.BlockID
	id   byzco.BlockID
	prev byzco.BlockID
	ref  *SignedData
}

type depgraph struct {
	await   map[byzco.BlockID][]byzco.BlockID
	bmu     sync.Mutex // protects blocks
	blocks  []*blockData
	cond    *sync.Cond // protects in
	ctx     context.Context
	icache  map[byzco.BlockID]bool
	in      []*blockData
	mu      sync.RWMutex // protects icache, tcache
	pending map[byzco.BlockID]*blockData
	self    uint64
	store   *store
	tcache  map[byzco.BlockID]bool
}

func (d *depgraph) actuallyIncluded(id byzco.BlockID) {
	d.mu.Lock()
	delete(d.tcache, id)
	d.mu.Unlock()
}

func (d *depgraph) add(info *blockData) {
	d.cond.L.Lock()
	d.in = append(d.in, info)
	d.cond.L.Unlock()
	d.cond.Signal()
}

func (d *depgraph) addPending(block *blockData, deps []byzco.BlockID) {
	d.pending[block.id] = block
	for _, dep := range deps {
		await, exists := d.await[dep]
		if exists {
			exists = false
			for _, id := range await {
				if id == block.id {
					exists = true
					break
				}
			}
			if !exists {
				d.await[dep] = append(await, block.id)
			}
		} else {
			d.await[dep] = []byzco.BlockID{block.id}
		}
	}
}

func (d *depgraph) getBlocks(limit int) []*blockData {
	d.bmu.Lock()
	idx := 0
	total := 0
	for _, block := range d.blocks {
		total += block.ref.Size()
		if total > limit {
			break
		}
		idx++
	}
	blocks := d.blocks[:idx]
	d.blocks = d.blocks[idx:]
	d.bmu.Unlock()
	return blocks
}

func (d *depgraph) isIncluded(id byzco.BlockID) bool {
	d.mu.RLock()
	inc, exists := d.tcache[id]
	if !exists {
		inc, exists = d.icache[id]
	}
	d.mu.RUnlock()
	if exists {
		return inc
	}
	inc, err := d.store.isIncluded(id)
	if err != nil {
		log.Fatal("Couldn't check if block has been included", fld.Err(err))
	}
	d.mu.Lock()
	d.icache[id] = inc
	d.mu.Unlock()
	return inc
}

func (d *depgraph) markIncluded(id byzco.BlockID) {
	d.mu.Lock()
	d.icache[id] = true
	d.tcache[id] = true
	d.mu.Unlock()
}

func (d *depgraph) process() {
	i := 0
	for {
		// Prune the included cache every 100 iterations.
		i++
		if i%100 == 0 {
			d.mu.Lock()
			if len(d.icache) > 1000 {
				ncache := map[byzco.BlockID]bool{}
				j := 0
				for k, v := range d.icache {
					ncache[k] = v
					j++
					if j == 1000 {
						break
					}
				}
				d.icache = ncache
			}
			d.mu.Unlock()
		}
		d.cond.L.Lock()
		for len(d.in) == 0 {
			d.cond.Wait()
			select {
			case <-d.ctx.Done():
				d.cond.L.Unlock()
				return
			default:
			}
		}
		info := d.in[0]
		d.in = d.in[1:]
		d.cond.L.Unlock()
		if !d.processBlock(info) {
			continue
		}
		first := true
		processed := []byzco.BlockID{info.id}
		seen := map[byzco.BlockID]bool{
			info.id: true,
		}
		for len(processed) > 0 {
			next := processed[0]
			processed = processed[1:]
			for _, revdep := range d.await[next] {
				if seen[revdep] {
					continue
				} else if d.processBlock(d.pending[revdep]) {
					processed = append(processed, revdep)
					seen = map[byzco.BlockID]bool{}
				} else {
					seen[revdep] = true
				}
			}
			delete(d.await, next)
			if first {
				first = false
			} else {
				delete(d.pending, next)
			}
		}
	}
}

func (d *depgraph) processBlock(block *blockData) bool {
	// Skip full processing of any blocks that have already been included into
	// one of our blocks.
	if d.isIncluded(block.id) {
		return true
	}
	// Check if all the referenced blocks have been included already.
	var deps []byzco.BlockID
	if block.prev.Valid() {
		if !d.isIncluded(block.prev) {
			log.Debug("Missing dependency", fld.BlockID(block.id), log.String("dep", block.prev.String()))
			deps = append(deps, block.prev)
		}
	}
	for _, dep := range block.deps {
		if dep.Node == d.self {
			continue
		}
		if !d.isIncluded(dep) {
			log.Debug("Missing dependency", fld.BlockID(block.id), log.String("dep", dep.String()))
			deps = append(deps, dep)
		}
	}
	// Mark the block as pending if any of the referenced blocks, including the
	// previous block, haven't been included.
	if len(deps) > 0 {
		d.addPending(block, deps)
		return false
	}
	// Mark the block as included and queue it for actual inclusion.
	d.markIncluded(block.id)
	d.bmu.Lock()
	d.blocks = append(d.blocks, block)
	d.bmu.Unlock()
	return true
}

func (d *depgraph) release() {
	<-d.ctx.Done()
	d.cond.Signal()
}
