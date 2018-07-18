package transactor // import "chainspace.io/prototype/service/transactor"
import (
	"chainspace.io/prototype/log"

	"go.uber.org/zap"
)

type SBACDecisions uint8

const (
	Accept SBACDecisions = iota

	Reject
	Abort
)

type StateTable struct {
	actions     map[State]Action
	transitions map[StateTransition]Transition
	onEvent     func(tx *TxDetails, event *Event) error
}

type StateTransition struct {
	From State
	To   State
}

type TxDetails struct {
	CheckersEvidences map[uint64][]byte
	Consensus1        *ConsensusTransaction
	Consensus2        *ConsensusTransaction
	ConsensusCommit   *ConsensusTransaction
	CommitDecisions   map[uint64]SBACDecision
	ID                []byte
	Phase1Decisions   map[uint64]SBACDecision
	Phase2Decisions   map[uint64]SBACDecision
	Raw               []byte
	Result            chan bool
	Tx                *Transaction
	HashID            uint32
}

// Action specify an action to execute when a new event is triggered.
// it returns a State, which will be either the new actual state, the next state
// which may required a transition from the current state to the new one (see the transition
// table
type Action func(tx *TxDetails) (State, error)

// Transition are called when the state is change from a current state to a new one.
// return a State which may involved a new transition as well.
type Transition func(tx *TxDetails) (State, error)

type Event struct {
	msg    *SBACMessage
	peerID uint64
}

type StateMachine struct {
	txDetails *TxDetails
	events    chan *Event
	state     State
	table     *StateTable
}

func (sm *StateMachine) Reset() {
	sm.state = StateWaitingForConsensus1
}

func (sm *StateMachine) State() State {
	return sm.state
}

func (sm *StateMachine) applyTransition(transitionTo State) error {
	for {
		txtransition := StateTransition{sm.state, transitionTo}
		f, ok := sm.table.transitions[txtransition]
		if !ok {
			// no more transitions available, this is not an error
			return nil
		}

		log.Info("applying transition",
			zap.Uint32("id", sm.txDetails.HashID),
			zap.String("old_state", sm.state.String()),
			zap.String("new_state", transitionTo.String()),
		)
		nextstate, err := f(sm.txDetails)
		if err != nil {
			log.Error("unable to apply transition",
				zap.Uint32("id", sm.txDetails.HashID),
				zap.String("old_state", sm.state.String()),
				zap.String("new_state", transitionTo.String()),
				zap.Error(err),
			)
			return err
		}
		sm.state = transitionTo
		transitionTo = nextstate
	}
}

func (sm *StateMachine) moveState() error {
	for {
		// first try to execute an action if possible with the current state
		action, ok := sm.table.actions[sm.state]
		if !ok {
			log.Error("unable to find an action to map with the current state",
				zap.String("state", sm.state.String()))
			return nil
		}
		log.Info("applying action",
			zap.Uint32("id", sm.txDetails.HashID),
			zap.String("state", sm.state.String()),
		)
		newstate, err := action(sm.txDetails)
		if err != nil {
			log.Error("unable to execute action", zap.Error(err))
			return err
		}
		if newstate == sm.state {
			// action returned the same state, we can return now as this action is not ready to be completed
			// although this is not an error
			return nil
		}
		// action succeed, we try to find a transition, if any we apply it, if none available, just set the new state.
		txtransition := StateTransition{sm.state, newstate}
		// if a transition exist for the new state, apply it
		if _, ok := sm.table.transitions[txtransition]; ok {
			err = sm.applyTransition(newstate)
			if err != nil {
				log.Error("unable to apply transition", zap.Error(err))
			}
		} else {
			// else save the new state directly
			sm.state = newstate
		}
	}
}

func (sm *StateMachine) run() {
	for e := range sm.events {
		log.Info("processing new event",
			zap.Uint32("id", sm.txDetails.HashID),
			zap.String("state", sm.state.String()),
			zap.Uint64("peer.id", e.peerID),
		)
		sm.table.onEvent(sm.txDetails, e)
		if sm.state == StateSucceeded || sm.state == StateAborted {
			log.Info("statemachine reach end", zap.String("final_state", sm.state.String()))
			return
		}
		err := sm.moveState()
		if err != nil {
			log.Error("something happend while moving states", zap.Error(err))
		}

	}
}

func (sm *StateMachine) OnEvent(e *Event) {
	sm.events <- e
}

func NewStateMachine(table *StateTable, txDetails *TxDetails, initialState State) *StateMachine {
	log.Info("starting new statemachine", zap.Uint32("tx.id", txDetails.HashID))
	sm := &StateMachine{
		events:    make(chan *Event, 1000),
		state:     initialState,
		table:     table,
		txDetails: txDetails,
	}
	go sm.run()
	return sm
}

func NewTxDetails(txID, raw []byte, tx *Transaction, evidences map[uint64][]byte) *TxDetails {
	return &TxDetails{
		CheckersEvidences: evidences,
		CommitDecisions:   map[uint64]SBACDecision{},
		ID:                txID,
		Phase1Decisions:   map[uint64]SBACDecision{},
		Phase2Decisions:   map[uint64]SBACDecision{},
		Raw:               raw,
		Result:            make(chan bool),
		Tx:                tx,
		HashID:            ID(txID),
	}
}
