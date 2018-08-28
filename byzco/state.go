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
	hnvState
	preparedState
	prepreparedState
	viewState
	viewchangedState
)

type commit struct {
	hash   string
	node   uint64
	round  uint64
	sender uint64
	view   uint32
}

func (c commit) kind() messageKind {
	return commitMsg
}

func (c commit) noderound() (uint64, uint64) {
	return c.node, c.round
}

func (c commit) String() string {
	return fmt.Sprintf(
		"commit{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		c.node, c.round, c.view, fmtHash(c.hash), c.sender,
	)
}

func (c commit) pre() preprepare {
	return preprepare{hash: c.hash, node: c.node, round: c.round, view: c.view}
}

type entry struct {
	block BlockID
	deps  []BlockID
	prev  BlockID
}

type final struct {
	node  uint64
	round uint64
}

func (f final) getRound() uint64 {
	return f.round
}

func (f final) skind() stateDataKind {
	return finalState
}

type hnv struct {
	node  uint64
	round uint64
	view  uint32
}

func (h hnv) getRound() uint64 {
	return h.round
}

func (h hnv) skind() stateDataKind {
	return hnvState
}

type message interface {
	kind() messageKind
	noderound() (uint64, uint64)
	String() string
}

type messageKind uint8

func (m messageKind) String() string {
	switch m {
	case commitMsg:
		return "commit"
	case newviewMsg:
		return "new-view"
	case prepareMsg:
		return "prepare"
	case preprepareMsg:
		return "pre-prepare"
	case unknownMsg:
		return "unknown"
	case viewchangeMsg:
		return "view-change"
	default:
		panic(fmt.Errorf("byzco: unknown message kind: %d", m))
	}
}

type newview struct {
	hash   string
	node   uint64
	round  uint64
	sender uint64
	view   uint32
}

func (n newview) kind() messageKind {
	return newviewMsg
}

func (n newview) noderound() (uint64, uint64) {
	return n.node, n.round
}

func (n newview) String() string {
	return fmt.Sprintf(
		"new-view{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		n.node, n.round, n.view, fmtHash(n.hash), n.sender,
	)
}

type noderound struct {
	node  uint64
	round uint64
}

type prepare struct {
	hash   string
	node   uint64
	round  uint64
	sender uint64
	view   uint32
}

func (p prepare) kind() messageKind {
	return prepareMsg
}

func (p prepare) noderound() (uint64, uint64) {
	return p.node, p.round
}

func (p prepare) pre() preprepare {
	return preprepare{hash: p.hash, node: p.node, round: p.round, view: p.view}
}

func (p prepare) String() string {
	return fmt.Sprintf(
		"prepare{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		p.node, p.round, p.view, fmtHash(p.hash), p.sender,
	)
}

type prepared struct {
	node  uint64
	round uint64
	view  uint32
}

func (p prepared) getRound() uint64 {
	return p.round
}

func (p prepared) skind() stateDataKind {
	return preparedState
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

func (p preprepare) noderound() (uint64, uint64) {
	return p.node, p.round
}

func (p preprepare) String() string {
	return fmt.Sprintf(
		"pre-prepare{node: %d, round: %d, view: %d, hash: '%s'}",
		p.node, p.round, p.view, fmtHash(p.hash),
	)
}

type preprepared struct {
	node  uint64
	round uint64
	view  uint32
}

func (p preprepared) getRound() uint64 {
	return p.round
}

func (p preprepared) skind() stateDataKind {
	return prepreparedState
}

type state struct {
	bitsets  map[preprepare]*bitset
	data     stateKV
	delay    map[uint64]uint64
	final    map[noderound]string
	out      []message
	timeout  uint64
	timeouts map[uint64][]timeout
}

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
		bitsets := map[preprepare]*bitset{}
		for k, v := range s.bitsets {
			if k.round < minround {
				continue
			}
			bitsets[k] = v.clone()
		}
		n.bitsets = bitsets
	}
	if s.data != nil {
		data := map[stateData]interface{}{}
		for k, v := range s.data {
			if k.getRound() < minround {
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
		final := map[noderound]string{}
		for k, v := range s.final {
			if k.round < minround {
				continue
			}
			final[k] = v
		}
		n.final = final
	}
	var out []message
	for _, msg := range s.out {
		_, r := msg.noderound()
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

func (s *state) getBitset(size int, pp preprepare) *bitset {
	if s.bitsets == nil {
		b := newBitset(size)
		s.bitsets = map[preprepare]*bitset{
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

func (s *state) getOutput() []message {
	if s == nil {
		return nil
	}
	return s.out
}

func (s *state) getView(node uint64, round uint64) uint32 {
	if val, exists := s.data[view{node: node, round: round}]; exists {
		return val.(uint32)
	}
	if s.data == nil {
		s.data = map[stateData]interface{}{
			view{node: node, round: round}: uint32(0),
		}
	} else {
		s.data[view{node: node, round: round}] = uint32(0)
	}
	return 0
}

type stateData interface {
	getRound() uint64
	skind() stateDataKind
}

type stateDataKind uint8

func (s stateDataKind) String() string {
	switch s {
	case finalState:
		return "final"
	case hnvState:
		return "hnv"
	case preparedState:
		return "prepared"
	case prepreparedState:
		return "preprepared"
	case unknownState:
		return "unknown"
	case viewState:
		return "viewState"
	case viewchangedState:
		return "viewchanged"
	default:
		panic(fmt.Errorf("byzco: unknown status data kind: %d", s))
	}
}

type stateKV map[stateData]interface{}

type timeout struct {
	node  uint64
	round uint64
	view  uint32
}

type view struct {
	node  uint64
	round uint64
}

func (v view) getRound() uint64 {
	return v.round
}

func (v view) skind() stateDataKind {
	return viewState
}

type viewchange struct {
	hash   string
	node   uint64
	round  uint64
	sender uint64
	view   uint32
}

func (v viewchange) kind() messageKind {
	return viewchangeMsg
}

func (v viewchange) noderound() (uint64, uint64) {
	return v.node, v.round
}

func (v viewchange) String() string {
	return fmt.Sprintf(
		"view-change{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		v.node, v.round, v.view, fmtHash(v.hash), v.sender,
	)
}

type viewchanged struct {
	node  uint64
	round uint64
	view  uint32
}

func (v viewchanged) getRound() uint64 {
	return v.round
}

func (v viewchanged) skind() stateDataKind {
	return viewchangedState
}

func diff(a uint64, b uint64) uint64 {
	if a >= b {
		return a - b
	}
	return b - a
}

func fmtHash(v string) string {
	if v == "" {
		return ""
	}
	return fmt.Sprintf("%X", v[6:12])
}
