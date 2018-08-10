package byzco

import (
	"chainspace.io/prototype/log"
)

type instance struct {
	hash   string
	log    *log.Logger
	node   uint64
	round  uint64
	states map[BlockID]*state
}

func (i *instance) process(e entry) {

	var s *state
	if e.prev.Valid() {
		s = i.states[e.prev].clone()
	} else {
		s = &state{
			data:     map[stateData]interface{}{},
			delay:    map[uint64]uint64{},
			final:    map[noderound]string{},
			timeouts: map[uint64][]timeout{},
		}
	}

	node, round, hash := e.block.Node, e.block.Round, e.block.Hash
	msgs := []message{preprepare{
		hash:  hash,
		node:  node,
		round: round,
		view:  0,
	}}
	out := []message{msgs[0]}

	for _, dep := range e.deps {
		s.delay[dep.Node] = diff(round, dep.Round)
	}

	tval := uint64(1)
	for _, val := range s.delay {
		if val > tval {
			tval = val
		}
	}
	s.timeout = tval * 10

	for _, tmout := range s.timeouts[round] {
		_, exists := s.data[final{node: tmout.node, round: tmout.round}]
		if exists {
			continue
		}
	}

	s.out = out
	i.states[e.block] = s

}

func diff(a uint64, b uint64) uint64 {
	if a >= b {
		return a - b
	}
	return b - a
}
