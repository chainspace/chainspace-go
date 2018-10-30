package sbac

import (
	"sync"
	"time"

	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
)

type StateMachineScheduler struct {
	mu                   sync.Mutex
	states               map[string]*StateMachine
	consensusEventAction ConsensusEventAction
	sbacEventAction      SBACEventAction
	table                *StateTable
}

func NewStateMachineScheduler(
	cea ConsensusEventAction, sea SBACEventAction, table *StateTable,
) *StateMachineScheduler {
	return &StateMachineScheduler{
		states:               map[string]*StateMachine{},
		consensusEventAction: cea,
		sbacEventAction:      sea,
		table:                table,
	}
}

func (s *StateMachineScheduler) newCfg(
	detail *DetailTx, initialState State) *StateMachineConfig {
	return &StateMachineConfig{
		ConsensusAction: s.consensusEventAction,
		SBACAction:      s.sbacEventAction,
		Table:           s.table,
		Detail:          detail,
		InitialState:    initialState,
	}
}

func (s *StateMachineScheduler) Add(detail *DetailTx, initialState State) *StateMachine {
	cfg := s.newCfg(detail, initialState)
	s.mu.Lock()
	sm := NewStateMachine(cfg)
	s.states[string(detail.ID)] = sm
	s.mu.Unlock()
	return sm
}

func (s *StateMachineScheduler) Get(txID []byte) (*StateMachine, bool) {
	s.mu.Lock()
	sm, ok := s.states[string(txID)]
	s.mu.Unlock()
	return sm, ok
}

func (s *StateMachineScheduler) GetOrCreate(
	detail *DetailTx, initialState State) *StateMachine {
	s.mu.Lock()
	defer s.mu.Unlock()
	sm, ok := s.states[string(detail.ID)]
	if ok {
		return sm
	}
	cfg := s.newCfg(detail, initialState)
	sm = NewStateMachine(cfg)
	s.states[string(detail.ID)] = sm
	return sm
}

// FIXME(): reimplement this properly
func (s *StateMachineScheduler) StatesReport() []*StateReport {
	s.mu.Lock()
	for _, v := range s.states {
		_ = v
		// sr = append(sr, v.StateReport())
	}
	s.mu.Unlock()
	return nil
}

func (s *StateMachineScheduler) RunGC() {
	for {
		time.Sleep(1 * time.Second)
		s.mu.Lock()
		for k, v := range s.states {
			if v.State() == StateAborted || v.State() == StateSucceeded {
				if log.AtDebug() {
					log.Debug("removing statemachine", log.String("finale_state", v.State().String()), fld.TxID(ID([]byte(k))))
				}
				// v.Close()
				delete(s.states, k)
			}
		}
		s.mu.Unlock()
	}
}
