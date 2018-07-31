package byzco

import (
	"testing"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
)

func TestInstances(t *testing.T) {
	actions := map[status]handler{
		initialState: func(i *instance) (status, error) {
			i.log.Info("Entering initialState")
			return fooState, nil
		},
		fooState: func(i *instance) (status, error) {
			i.log.Info("Entering fooState")
			return fooState, nil
		},
	}
	transitions := map[transition]handler{
		{initialState, fooState}: func(i *instance) (status, error) {
			i.log.Info("Transitioning from initialState to fooState")
			return fooState, nil
		},
		{fooState, completedState}: func(i *instance) (status, error) {
			i.log.Info("Transitioning from fooState to completedState")
			return completedState, nil
		},
	}
	log.ToConsole(log.DebugLevel)
	node := uint64(1)
	round := uint64(1)
	i := &instance{
		actions:     actions,
		log:         log.With(fld.NodeID(node), fld.Round(round)),
		node:        node,
		round:       round,
		status:      initialState,
		transitions: transitions,
	}

	i.handleEvent(&event{})
	if i.status != completedState {
		t.Fatalf("Unexpected end state: got %s, expected %s", i.status, completedState)
	}
}
