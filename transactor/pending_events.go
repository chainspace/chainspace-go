package transactor

import "sync"

type pendingEvents struct {
	cond   *sync.Cond
	events []*Event
	mu     sync.Mutex
	cb     func(*Event) bool
}

func (pe *pendingEvents) OnEvent(e *Event) {
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
		e := pe.events[0]
		pe.events = pe.events[1:]
		pe.mu.Unlock()
		if !pe.cb(e) {
			pe.OnEvent(e)
		}

	}

}

func NewPendingEvents(cb func(*Event) bool) *pendingEvents {
	pe := &pendingEvents{
		events: []*Event{},
		cb:     cb,
	}
	pe.cond = sync.NewCond(&pe.mu)
	return pe
}
