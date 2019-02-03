package sbac

import (
	"bytes"
	fmt "fmt"
	"sync"

	"chainspace.io/chainspace-go/internal/log"
	"chainspace.io/chainspace-go/internal/log/fld"
)

type StateTable struct {
	actions     map[State]Action
	transitions map[StateTransition]Transition
}

type StateTransition struct {
	From State
	To   State
}

type SignedDecision struct {
	Decision  SBACDecision
	Signature []byte
}

// Action specify an action to execute when a new event is triggered.
// it returns a State, which will be either the new actual state, the next state
// which may required a transition from the current state to the new one (see the transition
// table
type Action func(s *States) (State, error)

// Transition are called when the state is change from a current state to a new one.
// return a State which may involved a new transition as well.
type Transition func(s *States) (State, error)

type StateMachineConfig struct {
	ConsensusAction ConsensusEventAction
	SBACAction      SBACEventAction

	Table        *StateTable
	Detail       *DetailTx
	InitialState State
}

type DetailTx struct {
	ID        []byte
	RawTx     []byte
	Tx        *Transaction
	Evidences map[uint64][]byte
	HashID    uint32
}

type States struct {
	consensus map[ConsensusOp]*ConsensusStateMachine
	sbac      map[SBACOp]*SBACStateMachine
	detail    *DetailTx
}

type StateMachine struct {
	table  *StateTable
	mu     sync.Mutex
	events *pendingEvents
	states *States
	state  State
}

func (sm *StateMachine) StateReport() *StateReport {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s := StateReport{
		HashID:          sm.states.detail.HashID,
		State:           sm.state.String(),
		CommitDecisions: map[uint64]bool{},
		Phase1Decisions: map[uint64]bool{},
		Phase2Decisions: map[uint64]bool{},
		PendingEvents:   int32(sm.events.Len()),
	}
	for k, v := range sm.states.sbac[SBACOp_Commit].GetDecisions() {
		s.CommitDecisions[k] = v.Decision == SBACDecision_ACCEPT
	}
	for k, v := range sm.states.sbac[SBACOp_Phase1].GetDecisions() {
		s.Phase1Decisions[k] = v.Decision == SBACDecision_ACCEPT
	}
	for k, v := range sm.states.sbac[SBACOp_Phase2].GetDecisions() {
		s.Phase2Decisions[k] = v.Decision == SBACDecision_ACCEPT
	}
	return &s
}

func (sm *StateMachine) State() State {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.state
}

func (sm *StateMachine) setState(newState State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state = newState
}

func (sm *StateMachine) applyTransition(transitionTo State) error {
	for {
		curState := sm.State()
		txtransition := StateTransition{curState, transitionTo}
		f, ok := sm.table.transitions[txtransition]
		if !ok {
			// no more transitions available, this is not an error
			return nil
		}

		log.Info("applying transition",
			log.Uint32("id", sm.states.detail.HashID),
			log.String("old_state", curState.String()),
			log.String("new_state", transitionTo.String()),
		)
		nextstate, err := f(sm.states)
		if err != nil {
			log.Error("unable to apply transition",
				log.Uint32("id", sm.states.detail.HashID),
				log.String("old_state", curState.String()),
				log.String("new_state", transitionTo.String()),
				fld.Err(err),
			)
			return err
		}
		sm.setState(transitionTo)
		transitionTo = nextstate
	}
}

func (sm *StateMachine) moveState() error {
	for {
		curState := sm.State()
		// first try to execute an action if possible with the current state
		action, ok := sm.table.actions[curState]
		if !ok {
			log.Error("unable to find an action to map with the current state",
				log.String("state", curState.String()), fld.TxID(sm.states.detail.HashID))
			return nil
		}
		log.Info("applying action",
			log.Uint32("id", sm.states.detail.HashID),
			log.String("state", curState.String()),
		)
		newstate, err := action(sm.states)
		if err != nil {
			log.Error("unable to execute action", fld.Err(err))
			return err
		}
		if newstate == sm.state {
			// action returned the same state, we can return now as this action is not ready to be completed
			// although this is not an error
			return nil
		}
		// action succeed, we try to find a transition, if any we apply it, if none available, just set the new state.
		txtransition := StateTransition{curState, newstate}
		// if a transition exist for the new state, apply it
		if _, ok := sm.table.transitions[txtransition]; ok {
			err = sm.applyTransition(newstate)
			if err != nil {
				log.Error("unable to apply transition", fld.Err(err))
			}
		} else {
			// else save the new state directly
			sm.setState(newstate)
		}
	}
}

func (sm *StateMachine) processEvent(rawe Event) error {
	if !bytes.Equal(rawe.TxID(), sm.states.detail.ID) {
		return fmt.Errorf("event linked to another transaction")
	}
	switch rawe.Kind() {
	case EventKindConsensus:
		e := rawe.(*ConsensusEvent)
		sm.states.consensus[e.data.Op].processEvent(sm.states, e)
		return nil
	case EventKindSBACMessage:
		e := rawe.(*SBACEvent)
		sm.states.sbac[e.msg.Op].processEvent(sm.states, e)
		return nil
	default:
		return fmt.Errorf("unknown event")
	}
}

func (sm *StateMachine) consumeEvent(e Event) bool {
	curState := sm.State()
	if log.AtDebug() {
		log.Info("processing new event",
			log.Uint32("id", sm.states.detail.HashID),
			log.String("state", curState.String()),
			fld.PeerID(e.PeerID()),
		)
	}
	// process the current event
	sm.processEvent(e)

	// then try to move the state from this point
	if curState == StateSucceeded || curState == StateAborted {
		if log.AtDebug() {
			log.Debug("statemachine reach end", log.String("final_state", sm.state.String()))
		}
		return true
	}
	err := sm.moveState()
	if err != nil {
		log.Error("something happend while moving states", fld.Err(err))
	}

	return true
}

func (sm *StateMachine) OnEvent(e Event) {
	sm.events.OnEvent(e)
}

func NewStateMachine(cfg *StateMachineConfig) *StateMachine {
	sm := &StateMachine{
		table: cfg.Table,
		states: &States{
			detail:    cfg.Detail,
			consensus: map[ConsensusOp]*ConsensusStateMachine{},
			sbac:      map[SBACOp]*SBACStateMachine{},
		},
		state: cfg.InitialState,
	}
	sm.states.consensus[ConsensusOp_Consensus1] =
		NewConsensuStateMachine(ConsensusOp_Consensus1, cfg.ConsensusAction)
	sm.states.consensus[ConsensusOp_Consensus2] =
		NewConsensuStateMachine(ConsensusOp_Consensus2, cfg.ConsensusAction)
	sm.states.consensus[ConsensusOp_ConsensusCommit] =
		NewConsensuStateMachine(ConsensusOp_ConsensusCommit, cfg.ConsensusAction)

	sm.states.sbac[SBACOp_Phase1] =
		NewSBACStateMachine(SBACOp_Phase1, cfg.SBACAction)
	sm.states.sbac[SBACOp_Phase2] =
		NewSBACStateMachine(SBACOp_Phase2, cfg.SBACAction)
	sm.states.sbac[SBACOp_Commit] =
		NewSBACStateMachine(SBACOp_Commit, cfg.SBACAction)
	sm.events = NewPendingEvents(sm.consumeEvent)
	go sm.events.Run()
	return sm
}
