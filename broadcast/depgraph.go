package broadcast

import (
	"bytes"
	"context"
	"sync"

	"chainspace.io/prototype/byzco"
	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"github.com/gogo/protobuf/proto"
)

type awaitBlock struct {
	await []byzco.BlockID
	block *blockData
}

type blockData struct {
	id    byzco.BlockID
	links []byzco.BlockID
	prev  []byte
	ref   *SignedData
}

type depgraph struct {
	await   map[byzco.BlockID][]byzco.BlockID
	cond    *sync.Cond
	ctx     context.Context
	icache  map[byzco.BlockID]bool
	in      []*blockData
	pending map[byzco.BlockID]*awaitBlock
	out     chan *blockData
	store   *store
}

func (d *depgraph) add(info *blockData) {
	d.in = append(d.in, info)
	d.cond.Signal()
}

func (d *depgraph) addPending(await []byzco.BlockID, block *blockData) {
	d.pending[block.id] = &awaitBlock{await, block}
	for _, ref := range await {
		ids, exists := d.await[ref]
		if exists {
			exists = false
			for _, id := range ids {
				if id == block.id {
					exists = true
					break
				}
			}
			if !exists {
				d.await[ref] = append(ids, block.id)
			}
		} else {
			d.await[ref] = []byzco.BlockID{block.id}
		}
	}
}

func (d *depgraph) getBlockData(id byzco.BlockID) *blockData {
	signed, err := d.store.getBlock(id)
	if err != nil {
		log.Fatal("Unable to load block data", fld.Err(err))
	}
	if signed == nil {
		return nil
	}
	data := &blockData{
		id: id,
	}
	block := &Block{}
	if err := proto.Unmarshal(signed.Data, block); err != nil {
		log.Fatal("Unable to decode signed block", fld.Err(err))
	}
	hasher := combihash.New()
	if _, err := hasher.Write(signed.Data); err != nil {
		log.Fatal("Unable to hash signed block data", fld.Err(err))
	}
	hash := hasher.Digest()
	ref := &BlockReference{
		Hash:  hash,
		Node:  block.Node,
		Round: block.Round,
	}
	enc, err := proto.Marshal(ref)
	if err != nil {
		log.Fatal("Unable to encode block reference", fld.Err(err))
	}
	var links []byzco.BlockID
	for _, sref := range block.References {
		ref := &BlockReference{}
		if err := proto.Unmarshal(sref.Data, ref); err != nil {
			log.Fatal("Unable to decode block reference", fld.Err(err))
		}
		links = append(links, byzco.BlockID{
			Hash:   string(ref.Hash),
			NodeID: ref.Node,
			Round:  ref.Round,
		})
	}
	data.links = links
	data.ref = &SignedData{
		Data:      enc,
		Signature: signed.Signature,
	}
	data.prev = block.Previous
	return data
}

func (d *depgraph) isIncluded(id byzco.BlockID) bool {
	inc, exists := d.icache[id]
	if exists {
		return inc
	}
	inc, err := d.store.isIncluded(id)
	if err != nil {
		log.Fatal("Couldn't check if block has been included", fld.Err(err))
	}
	d.icache[id] = inc
	return inc
}

func (d *depgraph) markIncluded(id byzco.BlockID) {
	d.icache[id] = true
}

func (d *depgraph) process() {
	i := 0
	for {
		// Prune the included cache every 100 loop.
		i++
		if i%100 == 0 {
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
		}
		d.cond.L.Lock()
		for len(d.in) == 0 {
			d.cond.Wait()
			select {
			case <-d.ctx.Done():
				return
			default:
			}
		}
		info := d.in[0]
		d.in = d.in[1:]
		d.cond.L.Unlock()
		d.processBlock(info, map[byzco.BlockID]bool{})
	}
}

func (d *depgraph) processBlock(block *blockData, seen map[byzco.BlockID]bool) bool {
	// seen[info.id] = true
	// Skip full processing of any blocks that have already been included into
	// one of our blocks.
	if d.isIncluded(block.id) {
		// d.tryAwaits(block.id, seen)
		return true
	}
	var await []byzco.BlockID
	// Validate the previous block except when it's the very first block.
	if !(block.id.Round == 1 && bytes.Equal(block.prev, genesis)) {
		prev := byzco.BlockID{
			Hash:   string(block.prev),
			NodeID: block.id.NodeID,
			Round:  block.id.Round - 1,
		}
		if !d.isIncluded(prev) {
			await = append(await, prev)
		}
	}
	// Validate all the referenced blocks.
	for _, ref := range block.links {
		if !d.isIncluded(ref) {
			await = append(await, ref)
		}
	}
	// Mark the block as pending if any of the referenced blocks, including
	// the previous block, haven't been included.
	if len(await) > 0 {
		d.addPending(await, block)
		return false
	}
	// Queue the block for inclusion and mark it as included.
	d.out <- block
	d.markIncluded(block.id)
	d.tryAwaits(block.id, seen)
	return true
}

func (d *depgraph) release() {
	<-d.ctx.Done()
	d.cond.Signal()
}

func (d *depgraph) tryAwaits(id byzco.BlockID, seen map[byzco.BlockID]bool) {
	await, exists := d.await[id]
	if !exists {
		return
	}
	// var (
	// 	nawait   []byzco.BlockID
	// 	toRemove []byzco.BlockID
	// )
	for _, ref := range await {
		if !seen[ref] {
			// if d.processBlock(d.pending[ref], seen) {
			// 	toRemove = append(toRemove, ref)
			// } else {
			// 	nawait = append(nawait, ref)
			// }
		}
	}
	// if len(nawait) == 0 {
	// 	for _, ref := range toRemove {
	// 		delete(d.pending, ref)
	// 	}
	// }
}
