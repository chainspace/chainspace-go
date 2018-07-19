package byzco

import (
	"log"
	"sync"
)

type event struct{}

// Instance represents a state machine for determining the block for a given node.
type instance struct {
	cond   *sync.Cond
	events []*event
	log    *log.Logger
	mu     sync.Mutex
	node   uint64
	round  uint64
}

func (i *instance) addEvent(e *event) {
	i.mu.Lock()
	i.events = append(i.events, e)
	i.mu.Unlock()
	i.cond.Signal()
}

func (i *instance) handleEvent(e *event) {
}

func (i *instance) run() {
	for {
		i.mu.Lock()
		for len(i.events) == 0 {
			i.cond.Wait()
		}
		e := i.events[0]
		i.events = i.events[1:]
		i.mu.Unlock()
		i.handleEvent(e)
	}
}

func newInstance(node uint64, round uint64) *instance {
	i := &instance{
		node:  node,
		round: round,
	}
	i.cond = sync.NewCond(&i.mu)
	return i
}
