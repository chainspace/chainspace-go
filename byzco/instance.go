package byzco

import (
	"context"
	"fmt"
	"sync"

	"chainspace.io/prototype/log"
)

const (
	initialState status = iota + 1
)

type action func(i *instance) (status, error)

type event struct{}

// Instance represents a state machine for determining the block for a given node.
type instance struct {
	cond   *sync.Cond
	ctx    context.Context
	events []*event
	log    *log.Logger
	mu     sync.Mutex
	node   uint64
	round  uint64
	status status
}

func (i *instance) addEvent(e *event) {
	i.mu.Lock()
	i.events = append(i.events, e)
	i.mu.Unlock()
	i.cond.Signal()
}

func (i *instance) handleEvent(e *event) {
}

func (i *instance) release() {
	<-i.ctx.Done()
	i.cond.Signal()
}

func (i *instance) run() {
	for {
		i.mu.Lock()
		for len(i.events) == 0 {
			i.cond.Wait()
			select {
			case <-i.ctx.Done():
				return
			default:
			}
		}
		e := i.events[0]
		i.events = i.events[1:]
		i.mu.Unlock()
		i.handleEvent(e)
	}
}

type stateTransition struct {
	from status
	to   status
}

type status uint8

func (s status) String() string {
	switch s {
	case 0:
		return "foo"
	default:
		panic(fmt.Errorf("byzco: unknown status: %d", s))
	}
}

type transition func(i *instance) (status, error)

func newInstance(node uint64, round uint64) *instance {
	i := &instance{
		node:  node,
		round: round,
	}
	i.cond = sync.NewCond(&i.mu)
	go i.release()
	go i.run()
	return i
}
