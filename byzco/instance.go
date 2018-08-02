package byzco

import (
	"fmt"
	"sync"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
)

const (
	unknownState status = iota
	committed
	inCommit
	inPrePrepare
	inPrepare
	initialState
)

type event struct{}

type handler func(i *instance) (status, error)

// Instance represents a state machine for determining the block for a given node.
type instance struct {
	actions     map[status]handler
	c           *controller
	cond        *sync.Cond
	events      []*event
	hash        string
	log         *log.Logger
	mu          sync.Mutex
	node        uint64
	perspective uint64
	round       uint64
	status      status
	transitions map[transition]handler
}

func (i *instance) addEvent(e *event) {
	i.mu.Lock()
	i.events = append(i.events, e)
	i.mu.Unlock()
	i.cond.Signal()
}

func (i *instance) handleEvent(e *event) {
	// First, process the event by updating any relevant state.

	// Then, run actions and transition handlers until we can progress no
	// further.
	for {
		if i.status == committed {
			return
		}
		action, exists := i.actions[i.status]
		if !exists {
			i.log.Fatal("Unable to find an action for the current state", fld.Status(i.status))
		}
		status, err := action(i)
		if err != nil {
			i.log.Fatal("Unable to run the action", fld.Status(i.status), fld.Err(err))
		}
		if status == i.status {
			return
		}
		for {
			handler, exists := i.transitions[transition{i.status, status}]
			if !exists {
				i.status = status
				break
			}
			next, err := handler(i)
			if err != nil {
				i.log.Fatal("Unable to run the transition handler", fld.FromState(i.status), fld.ToState(status), fld.Err(err))
			}
			i.status = status
			status = next
		}
	}
}

func (i *instance) release() {
	<-i.c.ctx.Done()
	i.cond.Signal()
}

func (i *instance) run() {
	for {
		i.mu.Lock()
		for len(i.events) == 0 {
			i.cond.Wait()
			select {
			case <-i.c.ctx.Done():
				i.mu.Unlock()
				return
			default:
			}
		}
		e := i.events[0]
		i.events = i.events[1:]
		i.mu.Unlock()
		i.handleEvent(e)
		if i.status == committed {
			i.c.callback(i.perspective, i.node, i.hash)
			break
		}
	}
}

type transition struct {
	from status
	to   status
}

type status uint8

func (s status) String() string {
	switch s {
	case unknownState:
		return "unknownState"
	case committed:
		return "committed"
	case inCommit:
		return "inCommit"
	case inPrePrepare:
		return "inPrePrepare"
	case inPrepare:
		return "inPrepare"
	case initialState:
		return "initialState"
	default:
		panic(fmt.Errorf("byzco: unknown status: %d", s))
	}
}

func newInstance(c *controller, perspective uint64, node uint64, round uint64) *instance {
	i := &instance{
		actions:     actions,
		c:           c,
		log:         log.With(fld.Round(round), fld.Perspective(perspective), fld.NodeID(node)),
		node:        node,
		perspective: perspective,
		round:       round,
		status:      initialState,
		transitions: transitions,
	}
	i.cond = sync.NewCond(&i.mu)
	go i.release()
	go i.run()
	return i
}
