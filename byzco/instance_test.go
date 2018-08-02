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
			return inPrePrepare, nil
		},
		inPrePrepare: func(i *instance) (status, error) {
			i.log.Info("Entering inPrePrepare")
			return committed, nil
		},
	}
	transitions := map[transition]handler{
		{initialState, inPrePrepare}: func(i *instance) (status, error) {
			i.log.Info("Transitioning from initialState to inPrePrepare")
			return inPrePrepare, nil
		},
		{inPrePrepare, committed}: func(i *instance) (status, error) {
			i.log.Info("Transitioning from inPrePrepare to committed")
			return committed, nil
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
	if i.status != committed {
		t.Fatalf("Unexpected end state: got %s, expected %s", i.status, committed)
	}
}
