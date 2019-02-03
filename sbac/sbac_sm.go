package sbac

import (
	"fmt"
	"sync"

	"chainspace.io/chainspace-go/internal/log"
	"chainspace.io/chainspace-go/internal/log/fld"
)

type StateSBAC uint8

const (
	StateSBACWaiting StateSBAC = iota
	StateSBACAccepted
	StateSBACRejected
)

func (e StateSBAC) String() string {
	switch e {
	case StateSBACWaiting:
		return "StateSBACWaiting"
	case StateSBACAccepted:
		return "StateSBACAccepted"
	case StateSBACRejected:
		return "StateSBACRejected"
	default:
		return "error"
	}
}

type SBACEventAction func(st *States, decisions map[uint64]SignedDecision, e *SBACEvent) (StateSBAC, error)

type SBACStateMachine struct {
	action    SBACEventAction
	mu        sync.Mutex
	msgs      map[uint64]*SBACMessage
	decisions map[uint64]SignedDecision
	phase     SBACOp
	state     StateSBAC
}

func (c *SBACStateMachine) State() StateSBAC {
	return c.state
}

func (c *SBACStateMachine) processEvent(st *States, e *SBACEvent) error {
	if e.Kind() != EventKindSBACMessage {
		return fmt.Errorf("SBACStateMachine, invalid EventKind(%v)",
			e.Kind().String())
	}

	if c.state != StateSBACWaiting {
		return fmt.Errorf("SBACStateMachine already finished, state(%v)",
			c.state.String())
	}

	var err error
	c.msgs[e.msg.PeerID] = e.msg
	c.SetDecision(e.PeerID(), SignedDecision{e.msg.Decision, e.msg.Signature})
	c.state, err = c.action(st, c.GetDecisions(), e)
	return err
}

func (c *SBACStateMachine) Phase() SBACOp {
	return c.phase
}

func (c *SBACStateMachine) Data() interface{} {
	return c.msgs
}

func (c *SBACStateMachine) GetDecisions() map[uint64]SignedDecision {
	c.mu.Lock()
	out := map[uint64]SignedDecision{}
	for k, v := range c.decisions {
		out[k] = v
	}
	c.mu.Unlock()
	return out
}

func (c *SBACStateMachine) SetDecision(n uint64, d SignedDecision) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.decisions[n] = d
}

func NewSBACStateMachine(phase SBACOp, action SBACEventAction) *SBACStateMachine {
	return &SBACStateMachine{
		action:    action,
		phase:     phase,
		state:     StateSBACWaiting,
		msgs:      map[uint64]*SBACMessage{},
		decisions: map[uint64]SignedDecision{},
	}
}

func (s *ServiceSBAC) onSBACEvent(
	st *States, decisions map[uint64]SignedDecision, e *SBACEvent) (StateSBAC, error) {
	shards := s.shardsInvolvedWithoutSelf(st.detail.Tx)
	var somePending bool
	// for each shards, get the nodes id, and checks if they answered
	// vtwotplusone := quorum2t1(s.shardSize)
	vtwotplusone := s.shardSize
	vtplusone := quorumt1(s.shardSize)
	for _, v := range shards {
		nodes := s.top.NodesInShard(v)
		var accepted uint64
		var rejected uint64
		for _, nodeID := range nodes {
			if d, ok := decisions[nodeID]; ok {
				if d.Decision == SBACDecision_ACCEPT {
					accepted += 1
					continue
				}
				rejected += 1
			}
		}
		if rejected >= vtplusone {
			if log.AtDebug() {
				log.Debug("transaction rejected",
					log.String("sbac.phase", e.msg.Op.String()),
					fld.TxID(st.detail.HashID),
					fld.PeerShard(v),
					log.Uint64("t+1", vtplusone),
					log.Uint64("rejected", rejected),
				)
			}
			return StateSBACRejected, nil
		}
		if accepted >= vtwotplusone {
			if log.AtDebug() {
				log.Debug("transaction accepted",
					log.String("sbac.phase", e.msg.Op.String()),
					fld.TxID(st.detail.HashID),
					fld.PeerShard(v),
					log.Uint64s("shards_involved", shards),
					log.Uint64("2t+1", vtwotplusone),
					log.Uint64("accepted", accepted),
				)
			}
			continue
		}
		somePending = true
	}

	if somePending {
		if log.AtDebug() {
			log.Debug("transaction pending, not enough answers from shards",
				fld.TxID(st.detail.HashID),
				log.String("sbac.phase", e.msg.Op.String()))
		}
		return StateSBACWaiting, nil
	}

	if log.AtDebug() {
		log.Debug("transaction accepted by all shards",
			fld.TxID(st.detail.HashID),
			log.String("sbac.phase", e.msg.Op.String()))
	}

	// verify signatures now
	for k, v := range decisions {
		// TODO(): what to do with nodes with invalid signature
		ok, err := s.verifyTransactionSignature(st.detail.Tx, v.Signature, k)
		if err != nil {
			log.Error("unable to verify signature",
				log.String("sbac.phase", e.msg.Op.String()),
				fld.TxID(st.detail.HashID),
				fld.Err(err))
		}
		if !ok {
			log.Error("invalid signature for a decision",
				log.String("sbac.phase", e.msg.Op.String()),
				fld.TxID(st.detail.HashID),
				fld.PeerID(k))
		}
	}

	return StateSBACAccepted, nil
}
