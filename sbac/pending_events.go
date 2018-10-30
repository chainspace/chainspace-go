package sbac // import "chainspace.io/prototype/sbac"

import (
	"sync"

	"context"
)

type pendingEvents struct {
	cancel func()
	cond   *sync.Cond
	ctx    context.Context
	events []Event
	mu     sync.Mutex
	cb     func(Event) bool
}

func (pe *pendingEvents) Len() int {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	return len(pe.events)
}

func (pe *pendingEvents) Close() {
	pe.cancel()
	// just sending a nil event, this will get canceled directly
	pe.OnEvent(nil)
}

func (pe *pendingEvents) OnEvent(e Event) {
	pe.mu.Lock()
	pe.events = append(pe.events, e)
	pe.mu.Unlock()
	pe.cond.Signal()
}

func (pe *pendingEvents) Run() {
	for {
		pe.mu.Lock()
		for len(pe.events) == 0 {
			pe.cond.Wait()
		}
		// check if context exited
		if pe.ctx.Err() != nil {
			return
		}
		e := pe.events[0]
		pe.events = pe.events[1:]
		pe.mu.Unlock()
		if !pe.cb(e) {
			pe.OnEvent(e)
		}
	}
}

func NewPendingEvents(cb func(Event) bool) *pendingEvents {
	ctx, cancel := context.WithCancel(context.Background())
	pe := &pendingEvents{
		cancel: cancel,
		cb:     cb,
		ctx:    ctx,
		events: []Event{},
	}
	pe.cond = sync.NewCond(&pe.mu)
	return pe
}
