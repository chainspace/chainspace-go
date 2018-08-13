package byzco

import (
	"fmt"
)

const (
	unknownMsg messageKind = iota
	preprepareMsg
	prepareMsg
	commitMsg
	viewchangeMsg
	newviewMsg
)

const (
	unknownState stateDataKind = iota
	finalState
)

type entry struct {
	block BlockID
	deps  []BlockID
	prev  BlockID
}

type commit struct {
}

func (c commit) kind() messageKind {
	return commitMsg
}

type final struct {
	node  uint64
	round uint64
}

func (f final) skind() stateDataKind {
	return finalState
}

type message interface {
	kind() messageKind
}

type messageKind uint8

func (m messageKind) String() string {
	switch m {
	case preprepareMsg:
		return "pre-prepare"
	case prepareMsg:
		return "prepare"
	case commitMsg:
		return "commit"
	case viewchangeMsg:
		return "view-change"
	case newviewMsg:
		return "new-view"
	case unknownMsg:
		return "unknown"
	default:
		panic(fmt.Errorf("byzco: unknown message kind: %d", m))
	}
}

type newview struct {
}

func (n newview) kind() messageKind {
	return newviewMsg
}

type noderound struct {
	node  uint64
	round uint64
}

type prepare struct {
	hash     string
	node     uint64
	receiver uint64
	round    uint64
	view     uint32
}

func (p prepare) kind() messageKind {
	return prepareMsg
}

type preprepare struct {
	hash  string
	node  uint64
	round uint64
	view  uint32
}

func (p preprepare) kind() messageKind {
	return preprepareMsg
}

type state struct {
	bitsets  map[uint64]*bitset
	data     map[stateData]interface{}
	delay    map[uint64]uint64
	final    map[noderound]string
	out      []message
	timeout  uint64
	timeouts map[uint64][]timeout
}

func (s *state) clone() *state {
	n := &state{
		timeout: s.timeout,
	}
	if s.bitsets != nil {
		bitsets := map[uint64]*bitset{}
		for k, v := range s.bitsets {
			bitsets[k] = v.clone()
		}
		n.bitsets = bitsets
	}
	if s.data != nil {
		data := map[stateData]interface{}{}
		for k, v := range s.data {
			data[k] = v
		}
		n.data = data
	}
	delay := map[uint64]uint64{}
	for k, v := range s.delay {
		delay[k] = v
	}
	n.delay = delay
	if s.final != nil {
		final := map[noderound]string{}
		for k, v := range s.final {
			final[k] = v
		}
		n.final = final
	}
	out := make([]message, len(s.out))
	copy(out, s.out)
	n.out = out
	timeouts := map[uint64][]timeout{}
	for k, v := range s.timeouts {
		timeouts[k] = v
	}
	n.timeouts = timeouts
	return n
}

type stateData interface {
	skind() stateDataKind
}

type stateDataKind uint8

func (s stateDataKind) String() string {
	switch s {
	case finalState:
		return "final"
	case unknownState:
		return "unknown"
	default:
		panic(fmt.Errorf("byzco: unknown status kind: %d", s))
	}
}

type timeout struct {
	node  uint64
	round uint64
	view  uint32
}

type viewchange struct {
}

func (v viewchange) kind() messageKind {
	return viewchangeMsg
}

func diff(a uint64, b uint64) uint64 {
	if a >= b {
		return a - b
	}
	return b - a
}

// type message struct {
// 	typ                   status
// 	view                  int
// 	hash                  string
// 	node                  uint64
// 	preMsgs               []message // only exists in view-change
// 	preparesOrViewChanges []message // only exists in view-change or new-view
// }

// view change (view, set of pre and prepares)
// new-view (view, set of view-changes)

// view         int
// in           []message
// out          []message
// timeoutNode  uint64
// timeoutRound uint64
