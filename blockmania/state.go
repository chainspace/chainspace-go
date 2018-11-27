package blockmania

import (
	"chainspace.io/prototype/blockmania/messages"
	"chainspace.io/prototype/blockmania/states"
)

type nodeRound struct {
	node  uint64
	round uint64
}

type state struct {
	bitsets  map[messages.PrePrepare]*bitset
	data     stateKV
	delay    map[uint64]uint64
	final    map[nodeRound]string
	out      []messages.Message
	timeout  uint64
	timeouts map[uint64][]timeout
}

// clone the current state and return it
func (s *state) clone(minround uint64) *state {
	if s == nil {
		return &state{
			timeouts: map[uint64][]timeout{},
		}
	}
	n := &state{
		timeout: s.timeout,
	}
	if s.bitsets != nil {
		bitsets := map[messages.PrePrepare]*bitset{}
		for k, v := range s.bitsets {
			if k.Round < minround {
				continue
			}
			bitsets[k] = v.clone()
		}
		n.bitsets = bitsets
	}
	if s.data != nil {
		data := map[states.StateData]interface{}{}
		for k, v := range s.data {
			if k.GetRound() < minround {
				continue
			}
			data[k] = v
		}
		n.data = data
	}
	if s.delay != nil {
		delay := map[uint64]uint64{}
		for k, v := range s.delay {
			delay[k] = v
		}
		n.delay = delay
	}
	if s.final != nil {
		final := map[nodeRound]string{}
		for k, v := range s.final {
			if k.round < minround {
				continue
			}
			final[k] = v
		}
		n.final = final
	}
	var out []messages.Message
	for _, msg := range s.out {
		_, r := msg.NodeRound()
		if r < minround {
			continue
		}
		out = append(out, msg)
	}
	n.out = out
	timeouts := map[uint64][]timeout{}
	for k, v := range s.timeouts {
		if k < minround {
			continue
		}
		timeouts[k] = v
	}
	n.timeouts = timeouts
	return n
}

func (s *state) getBitset(size int, pp messages.PrePrepare) *bitset {
	if s.bitsets == nil {
		b := newBitset(size)
		s.bitsets = map[messages.PrePrepare]*bitset{
			pp: b,
		}
		return b
	}
	if b, exists := s.bitsets[pp]; exists {
		return b
	}
	b := newBitset(size)
	s.bitsets[pp] = b
	return b
}

func (s *state) getOutput() []messages.Message {
	if s == nil {
		return nil
	}
	return s.out
}

func (s *state) getView(node uint64, round uint64) uint32 {
	if val, exists := s.data[states.View{Node: node, Round: round}]; exists {
		return val.(uint32)
	}
	if s.data == nil {
		s.data = map[states.StateData]interface{}{
			states.View{Node: node, Round: round}: uint32(0),
		}
	} else {
		s.data[states.View{Node: node, Round: round}] = uint32(0)
	}
	return 0
}

type stateKV map[states.StateData]interface{}

type timeout struct {
	node  uint64
	round uint64
	view  uint32
}

func diff(a uint64, b uint64) uint64 {
	if a >= b {
		return a - b
	}
	return b - a
}
