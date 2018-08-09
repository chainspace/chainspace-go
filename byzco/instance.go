package byzco

import (
	"sync"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
)

// instance determines the block for a given node.
type instance struct {
	cond        *sync.Cond
	events      []interface{}
	hash        string
	log         *log.Logger
	mu          sync.Mutex
	node        uint64
	perspective uint64
	round       uint64
	status      status
}

func (i *instance) addEvent(e interface{}) {
	i.mu.Lock()
	i.events = append(i.events, e)
	i.mu.Unlock()
	i.cond.Signal()
}

func (i *instance) release() {
	// <-i.c.ctx.Done()
	// i.cond.Signal()
}

func (i *instance) run() {
	for {
		i.mu.Lock()
		for len(i.events) == 0 {
			i.cond.Wait()
			// select {
			// case <-i.c.ctx.Done():
			// 	i.mu.Unlock()
			// 	return
			// default:
			// }
		}
		e := i.events[0]
		i.events = i.events[1:]
		i.mu.Unlock()
		_ = e
		// i.handleEvent(e)
		// if i.status == committed {
		// 	i.c.callback(i.perspective, i.node, i.hash)
		// 	break
		// }
	}
}

func newInstance(c *controller, perspective uint64, node uint64, round uint64) *instance {
	i := &instance{
		// c:           c,
		log:         log.With(fld.Round(round), fld.Perspective(perspective), fld.NodeID(node)),
		node:        node,
		perspective: perspective,
		round:       round,
		// status:      initialState,
	}
	i.cond = sync.NewCond(&i.mu)
	go i.release()
	go i.run()
	return i
}
