package blockmania

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
)

type blockInfo struct {
	data *BlockGraph
	max  uint64
}

// Config represents the configuration of a blockmania Graph.
type Config struct {
	LastInterpreted uint64
	Nodes           []uint64
	SelfID          uint64
	TotalNodes      uint64
}

// Graph represents the graph that is generated from the nodes in a shard
// broadcasting to each other.
type Graph struct {
	blocks     []*blockInfo
	cb         func(*Interpreted)
	ctx        context.Context
	entries    chan *BlockGraph
	max        map[BlockID]uint64
	mu         sync.Mutex // protects blocks, max
	nodeCount  int
	nodes      []uint64
	quorumf1   int
	quorum2f   int
	quorum2f1  int
	resolved   map[uint64]map[uint64]string
	round      uint64
	self       uint64
	states     map[BlockID]*state
	totalNodes uint64
}

func (g *Graph) deliver(node uint64, round uint64, hash string) {
	if round < g.round {
		return
	}
	hashes, exists := g.resolved[round]
	if exists {
		curhash, exists := hashes[node]
		if exists {
			if curhash != hash {
				log.Fatal("Mismatching block hash for delivery", fld.NodeID(node), fld.Round(round))
			}
		} else {
			if log.AtDebug() {
				log.Debug("Consensus achieved", fld.BlockID(BlockID{
					Hash:  hash,
					Node:  node,
					Round: round,
				}))
			}
			hashes[node] = hash
		}
	} else {
		hashes = map[uint64]string{
			node: hash,
		}
		g.resolved[round] = hashes
	}
	if round != g.round {
		return
	}
	if len(hashes) == g.nodeCount {
		g.deliverRound(round, hashes)
	}
}

func (g *Graph) deliverRound(round uint64, hashes map[uint64]string) {
	var blocks []BlockID
	for node, hash := range hashes {
		if hash == "" {
			continue
		}
		blocks = append(blocks, BlockID{Hash: hash, Node: node, Round: round})
	}
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Hash < blocks[j].Hash
	})
	delete(g.resolved, round)
	g.mu.Lock()
	consumed := g.blocks[0].data.Block.Round - 1
	idx := 0
	for i, info := range g.blocks {
		if info.max > round {
			break
		}
		delete(g.max, info.data.Block)
		delete(g.states, info.data.Block)
		for _, dep := range info.data.Deps {
			delete(g.max, dep.Block)
			delete(g.states, dep.Block)
		}
		consumed++
		idx = i + 1
	}
	if idx > 0 {
		g.blocks = g.blocks[idx:]
	}
	g.round++
	if log.AtDebug() {
		log.Debug("Mem usage:", log.Int("g.max", len(g.max)), log.Int("g.states", len(g.states)),
			log.Int("g.blocks", len(g.blocks)))
	}
	g.mu.Unlock()
	g.cb(&Interpreted{
		Blocks:   blocks,
		Consumed: consumed,
		Round:    round,
	})
	hashes, exists := g.resolved[round+1]
	if exists && len(hashes) == g.nodeCount {
		g.deliverRound(round+1, hashes)
	}
}

func (g *Graph) process(e *entry) {

	var s *state
	if e.prev.Valid() {
		s = g.states[e.prev].clone(g.round)
	} else {
		s = &state{
			timeouts: map[uint64][]timeout{},
		}
	}

	if log.AtDebug() {
		log.Debug("Interpreting block", fld.BlockID(e.block))
	}

	node, round, hash := e.block.Node, e.block.Round, e.block.Hash
	out := []message{preprepare{
		hash:  hash,
		node:  node,
		round: round,
		view:  0,
	}}

	if len(e.deps) > 0 {
		if s.delay == nil {
			s.delay = map[uint64]uint64{}
		}
		for _, dep := range e.deps {
			s.delay[dep.Node] = diff(round, dep.Round) * 10
		}
	}

	tval := uint64(10)
	if len(s.delay) > g.quorum2f1 {
		vals := make([]uint64, len(s.delay))
		i := 0
		for _, val := range s.delay {
			vals[i] = val
			i++
		}
		sort.Slice(vals, func(i, j int) bool {
			return vals[i] < vals[j]
		})
		xval := vals[g.quorum2f]
		if xval > tval {
			tval = xval
		}
	} else {
		for _, val := range s.delay {
			if val > tval {
				tval = val
			}
		}
	}
	s.timeout = tval

	tround := round + tval
	for _, xnode := range g.nodes {
		s.timeouts[tround] = append(s.timeouts[tround], timeout{
			node:  xnode,
			round: round,
			view:  0,
		})
	}

	for _, tmout := range s.timeouts[round] {
		if _, exists := s.data[final{node: tmout.node, round: tmout.round}]; exists {
			// We've already reached finality
			continue
		}
		var v uint32
		skey := view{node: tmout.node, round: tmout.round}
		if sval, exists := s.data[skey]; exists {
			v = sval.(uint32)
		}
		if v > tmout.view {
			// We've already moved view
			continue
		}
		hval := ""
		if pval, exists := s.data[prepared{node: tmout.node, round: tmout.round, view: tmout.view}]; exists {
			hval = pval.(string)
		}
		if s.data == nil {
			s.data = stateKV{skey: v + 1}
		} else {
			s.data[skey] = v + 1
		}
		out = append(out, viewchange{
			hash:   hval,
			node:   tmout.node,
			round:  tmout.round,
			sender: node,
			view:   tmout.view + 1,
		})
	}

	idx := len(out)
	processed := map[message]bool{}
	out = append(out, g.processMessages(s, processed, node, node, e.block, out[:idx])...)
	for _, dep := range e.deps {
		if log.AtDebug() {
			log.Debug("Processing block dep", fld.BlockID(dep))
		}
		out = append(out, g.processMessages(s, processed, dep.Node, node, e.block, g.states[dep].getOutput())...)
	}
	s.out = out
	g.states[e.block] = s

}

func (g *Graph) processMessage(s *state, sender uint64, receiver uint64, origin BlockID, msg message) message {

	node, round := msg.noderound()
	if _, exists := s.data[final{node: node, round: round}]; exists {
		return nil
	}

	v := s.getView(node, round)
	if log.AtDebug() {
		log.Debug("Processing message from block", fld.BlockID(origin),
			log.String("message", msg.String()))
	}

	switch m := msg.(type) {

	case preprepare:
		// TODO: and valid view!
		if v != m.view {
			return nil
		}
		pp := preprepared{node: node, round: round, view: m.view}
		if _, exists := s.data[pp]; exists {
			return nil
		}
		// assert m.view == 0 || (nid, xround, xv, "HNV") in state
		b := s.getBitset(g.nodeCount, m)
		b.setPrepare(sender)
		b.setPrepare(receiver)
		if s.data == nil {
			s.data = stateKV{pp: m}
		} else {
			s.data[pp] = m
		}
		return prepare{hash: m.hash, node: node, round: round, sender: receiver, view: m.view}

	case prepare:
		if v > m.view {
			return nil
		}
		if v < m.view {
			// TODO: should we remember future messages?
			b := s.getBitset(g.nodeCount, m.pre())
			b.setPrepare(m.sender)
			return nil
		}
		// assert m.view == 0 || (nid, xround, xv, "HNV") in state
		b := s.getBitset(g.nodeCount, m.pre())
		b.setPrepare(m.sender)
		if log.AtDebug() {
			log.Debugf("Prepare count == %d", b.prepareCount())
		}
		if b.prepareCount() != g.quorum2f1 {
			return nil
		}
		if b.hasCommit(receiver) {
			return nil
		}
		b.setCommit(receiver)
		p := prepared{node: node, round: round, view: m.view}
		if _, exists := s.data[p]; !exists {
			if s.data == nil {
				s.data = stateKV{p: m.hash}
			} else {
				s.data[p] = m.hash
			}
		}
		// assert s.data[p] == m.hash
		return commit{hash: m.hash, node: node, round: round, sender: receiver, view: m.view}

	case commit:
		if v < m.view {
			return nil
		}
		b := s.getBitset(g.nodeCount, m.pre())
		b.setCommit(m.sender)
		if log.AtDebug() {
			log.Debugf("Commit count == %d", b.commitCount())
		}
		if b.commitCount() != g.quorum2f1 {
			return nil
		}
		nr := noderound{node, round}
		if _, exists := s.final[nr]; exists {
			// assert value == m.hash
			return nil
		}
		if s.final == nil {
			s.final = map[noderound]string{
				nr: m.hash,
			}
		} else {
			s.final[nr] = m.hash
		}
		g.deliver(node, round, m.hash)

	case viewchange:
		if v > m.view {
			return nil
		}
		var vcs map[uint64]string
		// TODO: check whether we should store the viewchanged by view number
		key := viewchanged{node: node, round: round, view: v}
		if val, exists := s.data[key]; exists {
			vcs = val.(map[uint64]string)
		} else {
			vcs = map[uint64]string{}
			if s.data == nil {
				s.data = stateKV{key: vcs}
			} else {
				s.data[key] = vcs
			}
		}
		vcs[m.sender] = m.hash
		if len(vcs) != g.quorum2f1 {
			return nil
		}
		// Increase the view number
		s.data[view{node: node, round: round}] = m.view
		var hash string
		for _, hval := range vcs {
			if hval != "" {
				if hash != "" && hval != hash {
					log.Fatal("Got multiple hashes in a view change",
						fld.NodeID(node), fld.Round(round),
						log.Digest("hash", []byte(hash)), log.Digest("hash.alt", []byte(hval)))
				}
				hash = hval
			}
		}
		return newview{
			hash: hash, node: node, round: round, sender: receiver, view: m.view,
		}

	case newview:
		if v > m.view {
			return nil
		}
		key := hnv{node: node, round: round, view: m.view}
		if _, exists := s.data[key]; exists {
			return nil
		}
		if s.data == nil {
			s.data = stateKV{}
		}
		s.data[view{node: node, round: round}] = m.view
		// TODO: timeout value could overflow uint64 if m.view is over 63 if using `1 << m.view`
		tval := origin.Round + s.timeout + 5 // uint64(10*m.view)
		s.timeouts[tval] = append(s.timeouts[tval], timeout{node: node, round: round, view: m.view})
		s.data[key] = true
		return preprepare{hash: m.hash, node: node, round: round, view: m.view}

	default:
		panic(fmt.Errorf("blockmania: unknown message kind to process: %s", msg.kind()))

	}

	return nil
}

func (g *Graph) processMessages(s *state, processed map[message]bool, sender uint64, receiver uint64, origin BlockID, msgs []message) []message {
	var out []message
	for _, msg := range msgs {
		if processed[msg] {
			continue
		}
		resp := g.processMessage(s, sender, receiver, origin, msg)
		processed[msg] = true
		if resp != nil {
			out = append(out, resp)
		}
	}
	for i := 0; i < len(out); i++ {
		msg := out[i]
		if processed[msg] {
			continue
		}
		resp := g.processMessage(s, sender, receiver, origin, msg)
		processed[msg] = true
		if resp != nil {
			out = append(out, resp)
		}
	}
	return out
}

func (g *Graph) run() {
	for {
		select {
		case data := <-g.entries:
			entries := make([]*entry, len(data.Deps))
			g.mu.Lock()
			max := data.Block.Round
			round := g.round
			for i, dep := range data.Deps {
				if log.AtDebug() {
					log.Debug("Dep:", fld.BlockID(dep.Block))
				}
				dmax := dep.Block.Round
				rcheck := false
				e := &entry{
					block: dep.Block,
					prev:  dep.Prev,
				}
				if dep.Block.Round != 1 {
					e.deps = make([]BlockID, len(dep.Deps)+1)
					e.deps[0] = dep.Prev
					copy(e.deps[1:], dep.Deps)
					pmax, exists := g.max[dep.Prev]
					if !exists {
						rcheck = true
					} else if pmax > dmax {
						dmax = pmax
					}
				} else {
					e.deps = dep.Deps
				}
				entries[i] = e
				for _, link := range dep.Deps {
					lmax, exists := g.max[link]
					if !exists {
						rcheck = true
					} else if lmax > dmax {
						dmax = lmax
					}
				}
				if rcheck && round > dmax {
					dmax = round
				}
				g.max[dep.Block] = dmax
				if dmax > max {
					max = dmax
				}
			}
			rcheck := false
			if data.Block.Round != 1 {
				pmax, exists := g.max[data.Prev]
				if !exists {
					rcheck = true
				} else if pmax > max {
					max = pmax
				}
			}
			if rcheck && round > max {
				max = round
			}
			g.max[data.Block] = max
			g.blocks = append(g.blocks, &blockInfo{
				data: data,
				max:  max,
			})
			g.mu.Unlock()
			for _, e := range entries {
				g.process(e)
			}
			self := &entry{
				block: data.Block,
				prev:  data.Prev,
			}
			self.deps = make([]BlockID, len(data.Deps)+1)
			self.deps[0] = data.Prev
			for i, dep := range data.Deps {
				self.deps[i+1] = dep.Block
			}
			g.process(self)
		case <-g.ctx.Done():
			return
		}
	}
}

// Add updates the graph and notifies the appropriate controllers.
func (g *Graph) Add(data *BlockGraph) {
	if log.AtDebug() {
		log.Debug("Adding block to graph", fld.BlockID(data.Block))
	}
	g.entries <- data
}

// New instantiates a Graph for use by the broadcast/consensus mechanism.
func New(ctx context.Context, cfg *Config, cb func(*Interpreted)) *Graph {
	f := (len(cfg.Nodes) - 1) / 3
	g := &Graph{
		blocks:     []*blockInfo{},
		cb:         cb,
		ctx:        ctx,
		entries:    make(chan *BlockGraph, 10000),
		max:        map[BlockID]uint64{},
		nodeCount:  len(cfg.Nodes),
		nodes:      cfg.Nodes,
		quorumf1:   f + 1,
		quorum2f:   (2 * f),
		quorum2f1:  (2 * f) + 1,
		resolved:   map[uint64]map[uint64]string{},
		round:      cfg.LastInterpreted + 1,
		self:       cfg.SelfID,
		states:     map[BlockID]*state{},
		totalNodes: cfg.TotalNodes,
	}
	go g.run()
	return g
}
