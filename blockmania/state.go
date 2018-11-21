package blockmania

import (
	"fmt"
)

const (
	unknownMsg messageKind = iota
	prePrepareMsg
	prepareMsg
	commitMsg
	viewChangedMsg
	newViewMsg
)

const (
	unknownState stateDataKind = iota
	finalState
	hnvState
	preparedState
	prePreparedState
	viewState
	viewChangedState
)

type entry struct {
	block BlockID
	deps  []BlockID
	prev  BlockID
}

// commit

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

func (c commit) nodeRound() (uint64, uint64) {
	return c.node, c.round
}

func (c commit) pre() prePrepare {
	return prePrepare{hash: c.hash, node: c.node, round: c.round, view: c.view}
}

func (c commit) String() string {
	return fmt.Sprintf(
		"commit{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		c.node, c.round, c.view, fmtHash(c.hash), c.sender,
	)
}

// final

type final struct {
	node  uint64
	round uint64
}

func (f final) getRound() uint64 {
	return f.round
}

func (f final) sdKind() stateDataKind {
	return finalState
}

// hnv

type hnv struct {
	node  uint64
	round uint64
	view  uint32
}

func (h hnv) getRound() uint64 {
	return h.round
}

func (h hnv) sdKind() stateDataKind {
	return hnvState
}

// message

type message interface {
	kind() messageKind
	nodeRound() (uint64, uint64)
	String() string
}

type messageKind uint8

func (m messageKind) String() string {
	switch m {
	case commitMsg:
		return "commit"
	case newViewMsg:
		return "new-view"
	case prepareMsg:
		return "prepare"
	case prePrepareMsg:
		return "pre-prepare"
	case unknownMsg:
		return "unknown"
	case viewChangedMsg:
		return "view-change"
	default:
		panic(fmt.Errorf("blockmania: unknown message kind: %d", m))
	}
}

// newView

type newView struct {
	hash   string
	node   uint64
	round  uint64
	sender uint64
	view   uint32
}

func (n newView) kind() messageKind {
	return newViewMsg
}

func (n newView) nodeRound() (uint64, uint64) {
	return n.node, n.round
}

func (n newView) String() string {
	return fmt.Sprintf(
		"new-view{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		n.node, n.round, n.view, fmtHash(n.hash), n.sender,
	)
}

// nodeRound

type nodeRound struct {
	node  uint64
	round uint64
}

// prepare

type prepare struct {
	hash   string
	node   uint64
	round  uint64
	sender uint64
	view   uint32
}

// prepare

func (p prepare) kind() messageKind {
	return prepareMsg
}

func (p prepare) nodeRound() (uint64, uint64) {
	return p.node, p.round
}

func (p prepare) pre() prePrepare {
	return prePrepare{hash: p.hash, node: p.node, round: p.round, view: p.view}
}

func (p prepare) String() string {
	return fmt.Sprintf(
		"prepare{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		p.node, p.round, p.view, fmtHash(p.hash), p.sender,
	)
}

// prepared

type prepared struct {
	node  uint64
	round uint64
	view  uint32
}

func (p prepared) getRound() uint64 {
	return p.round
}

func (p prepared) sdKind() stateDataKind {
	return preparedState
}

// prePrepare

type prePrepare struct {
	hash  string
	node  uint64
	round uint64
	view  uint32
}

func (p prePrepare) kind() messageKind {
	return prePrepareMsg
}

func (p prePrepare) nodeRound() (uint64, uint64) {
	return p.node, p.round
}

func (p prePrepare) String() string {
	return fmt.Sprintf(
		"pre-prepare{node: %d, round: %d, view: %d, hash: '%s'}",
		p.node, p.round, p.view, fmtHash(p.hash),
	)
}

// prePrepared

type prePrepared struct {
	node  uint64
	round uint64
	view  uint32
}

func (p prePrepared) getRound() uint64 {
	return p.round
}

func (p prePrepared) sdKind() stateDataKind {
	return prePreparedState
}

// state

type state struct {
	bitsets  map[prePrepare]*bitset
	data     stateKV
	delay    map[uint64]uint64
	final    map[nodeRound]string
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
		bitsets := map[prePrepare]*bitset{}
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
		final := map[nodeRound]string{}
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
		_, r := msg.nodeRound()
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

func (s *state) getBitset(size int, pp prePrepare) *bitset {
	if s.bitsets == nil {
		b := newBitset(size)
		s.bitsets = map[prePrepare]*bitset{
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
	sdKind() stateDataKind
}

// stateDataKind

type stateDataKind uint8

func (s stateDataKind) String() string {
	switch s {
	case finalState:
		return "final"
	case hnvState:
		return "hnv"
	case preparedState:
		return "prepared"
	case prePreparedState:
		return "preprepared"
	case unknownState:
		return "unknown"
	case viewState:
		return "viewState"
	case viewChangedState:
		return "viewchanged"
	default:
		panic(fmt.Errorf("blockmania: unknown status data kind: %d", s))
	}
}

type stateKV map[stateData]interface{}

type timeout struct {
	node  uint64
	round uint64
	view  uint32
}

// view

type view struct {
	node  uint64
	round uint64
}

func (v view) getRound() uint64 {
	return v.round
}

func (v view) sdKind() stateDataKind {
	return viewState
}

// viewChange

type viewChange struct {
	hash   string
	node   uint64
	round  uint64
	sender uint64
	view   uint32
}

func (v viewChange) kind() messageKind {
	return viewChangedMsg
}

func (v viewChange) nodeRound() (uint64, uint64) {
	return v.node, v.round
}

func (v viewChange) String() string {
	return fmt.Sprintf(
		"view-change{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		v.node, v.round, v.view, fmtHash(v.hash), v.sender,
	)
}

// viewChanged

type viewChanged struct {
	node  uint64
	round uint64
	view  uint32
}

func (v viewChanged) getRound() uint64 {
	return v.round
}

func (v viewChanged) sdKind() stateDataKind {
	return viewChangedState
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
