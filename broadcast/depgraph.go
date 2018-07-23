package broadcast

import (
	"context"
	"sync"
)

type depgraph struct {
	cond  *sync.Cond
	ctx   context.Context
	mu    sync.RWMutex
	queue []*blockInfo
	refs  chan *blockInfo
}

func (d *depgraph) add(info *blockInfo) {
	d.mu.Lock()
	d.queue = append(d.queue, info)
	d.mu.Unlock()
	d.cond.Signal()
}

func (d *depgraph) process() {
	for {
		select {
		case <-d.ctx.Done():
			return
		default:
		}
		d.mu.Lock()
		// NOTE(tav): It's possible for the goroutine to be blocking on the Wait
		// call indefinitely, even though the ctx has been cancelled, and thus
		// never exit the goroutine.
		for len(d.queue) == 0 {
			d.cond.Wait()
		}
		info := d.queue[0]
		d.queue = d.queue[1:]
		d.mu.Unlock()
		d.refs <- info
	}
}
