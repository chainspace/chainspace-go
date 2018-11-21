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

func (graph *Graph) deliver(node uint64, round uint64, hash string) {
	if round < graph.round {
		return
	}
	hashes, exists := graph.resolved[round]
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
		graph.resolved[round] = hashes
	}
	if round != graph.round {
		return
	}
	if len(hashes) == graph.nodeCount {
		graph.deliverRound(round, hashes)
	}
}

func (graph *Graph) deliverRound(round uint64, hashes map[uint64]string) {
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
	delete(graph.resolved, round)
	graph.mu.Lock()
	consumed := graph.blocks[0].data.Block.Round - 1
	idx := 0
	for i, info := range graph.blocks {
		if info.max > round {
			break
		}
		delete(graph.max, info.data.Block)
		delete(graph.states, info.data.Block)
		for _, dep := range info.data.Deps {
			delete(graph.max, dep.Block)
			delete(graph.states, dep.Block)
		}
		consumed++
		idx = i + 1
	}
	if idx > 0 {
		graph.blocks = graph.blocks[idx:]
	}
	graph.round++
	if log.AtDebug() {
		log.Debug("Mem usage:", log.Int("g.max", len(graph.max)), log.Int("g.states", len(graph.states)),
			log.Int("g.blocks", len(graph.blocks)))
	}
	graph.mu.Unlock()
	graph.cb(&Interpreted{
		Blocks:   blocks,
		Consumed: consumed,
		Round:    round,
	})
	hashes, exists := graph.resolved[round+1]
	if exists && len(hashes) == graph.nodeCount {
		graph.deliverRound(round+1, hashes)
	}
}

func (graph *Graph) findOrCreateState(e *entry) *state {
	var stat *state
	if e.prev.Valid() {
		stat = graph.states[e.prev].clone(graph.round)
	} else {
		stat = &state{
			timeouts: map[uint64][]timeout{},
		}
	}
	return stat
}

func (graph *Graph) process(ntry *entry) {

	var state = graph.findOrCreateState(ntry)

	if log.AtDebug() {
		log.Debug("Interpreting block", fld.BlockID(ntry.block))
	}

	node, round, hash := ntry.block.Node, ntry.block.Round, ntry.block.Hash
	out := []message{prePrepare{
		hash:  hash,
		node:  node,
		round: round,
		view:  0,
	}}

	if len(ntry.deps) > 0 {
		if state.delay == nil {
			state.delay = map[uint64]uint64{}
		}
		for _, dep := range ntry.deps {
			state.delay[dep.Node] = diff(round, dep.Round) * 10
		}
	}

	tval := uint64(10)
	if len(state.delay) > graph.quorum2f1 {
		vals := make([]uint64, len(state.delay))
		i := 0
		for _, val := range state.delay {
			vals[i] = val
			i++
		}
		sort.Slice(vals, func(i, j int) bool {
			return vals[i] < vals[j]
		})
		xval := vals[graph.quorum2f]
		if xval > tval {
			tval = xval
		}
	} else {
		for _, val := range state.delay {
			if val > tval {
				tval = val
			}
		}
	}
	state.timeout = tval

	tround := round + tval
	for _, xnode := range graph.nodes {
		state.timeouts[tround] = append(state.timeouts[tround], timeout{
			node:  xnode,
			round: round,
			view:  0,
		})
	}

	for _, tmout := range state.timeouts[round] {
		if _, exists := state.data[final{node: tmout.node, round: tmout.round}]; exists {
			// We've already reached finality
			continue
		}
		var v uint32
		skey := view{node: tmout.node, round: tmout.round}
		if sval, exists := state.data[skey]; exists {
			v = sval.(uint32)
		}
		if v > tmout.view {
			// We've already moved view
			continue
		}
		hval := ""
		if pval, exists := state.data[prepared{node: tmout.node, round: tmout.round, view: tmout.view}]; exists {
			hval = pval.(string)
		}
		if state.data == nil {
			state.data = stateKV{skey: v + 1}
		} else {
			state.data[skey] = v + 1
		}
		out = append(out, viewChange{
			hash:   hval,
			node:   tmout.node,
			round:  tmout.round,
			sender: node,
			view:   tmout.view + 1,
		})
	}

	idx := len(out)
	processed := map[message]bool{}
	out = append(out, graph.processMessages(state, processed, node, node, ntry.block, out[:idx])...)
	for _, dep := range ntry.deps {
		if log.AtDebug() {
			log.Debug("Processing block dep", fld.BlockID(dep))
		}
		out = append(out, graph.processMessages(state, processed, dep.Node, node, ntry.block, graph.states[dep].getOutput())...)
	}
	state.out = out
	graph.states[ntry.block] = state

}

func (graph *Graph) processMessage(s *state, sender uint64, receiver uint64, origin BlockID, msg message) message {

	node, round := msg.nodeRound()
	if _, exists := s.data[final{node: node, round: round}]; exists {
		return nil
	}

	v := s.getView(node, round)
	if log.AtDebug() {
		log.Debug("Processing message from block", fld.BlockID(origin),
			log.String("message", msg.String()))
	}

	switch m := msg.(type) {

	case prePrepare:
		// TODO: and valid view!
		if v != m.view {
			return nil
		}
		pp := prePrepared{node: node, round: round, view: m.view}
		if _, exists := s.data[pp]; exists {
			return nil
		}
		// assert m.view == 0 || (nid, xround, xv, "HNV") in state
		b := s.getBitset(graph.nodeCount, m)
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
			b := s.getBitset(graph.nodeCount, m.pre())
			b.setPrepare(m.sender)
			return nil
		}
		// assert m.view == 0 || (nid, xround, xv, "HNV") in state
		b := s.getBitset(graph.nodeCount, m.pre())
		b.setPrepare(m.sender)
		if log.AtDebug() {
			log.Debugf("Prepare count == %d", b.prepareCount())
		}
		if b.prepareCount() != graph.quorum2f1 {
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
		b := s.getBitset(graph.nodeCount, m.pre())
		b.setCommit(m.sender)
		if log.AtDebug() {
			log.Debugf("Commit count == %d", b.commitCount())
		}
		if b.commitCount() != graph.quorum2f1 {
			return nil
		}
		nr := nodeRound{node, round}
		if _, exists := s.final[nr]; exists {
			// assert value == m.hash
			return nil
		}
		if s.final == nil {
			s.final = map[nodeRound]string{
				nr: m.hash,
			}
		} else {
			s.final[nr] = m.hash
		}
		graph.deliver(node, round, m.hash)

	case viewChange:
		if v > m.view {
			return nil
		}
		var vcs map[uint64]string
		// TODO: check whether we should store the viewChanged by view number
		key := viewChanged{node: node, round: round, view: v}
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
		if len(vcs) != graph.quorum2f1 {
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
		return newView{
			hash: hash, node: node, round: round, sender: receiver, view: m.view,
		}

	case newView:
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
		return prePrepare{hash: m.hash, node: node, round: round, view: m.view}

	default:
		panic(fmt.Errorf("blockmania: unknown message kind to process: %s", msg.kind()))

	}

	return nil
}

func (graph *Graph) processMessages(s *state, processed map[message]bool, sender uint64, receiver uint64, origin BlockID, msgs []message) []message {
	var out []message
	for _, msg := range msgs {
		if processed[msg] {
			continue
		}
		resp := graph.processMessage(s, sender, receiver, origin, msg)
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
		resp := graph.processMessage(s, sender, receiver, origin, msg)
		processed[msg] = true
		if resp != nil {
			out = append(out, resp)
		}
	}
	return out
}

func (graph *Graph) run() {
	for {
		select {
		case data := <-graph.entries:
			entries := make([]*entry, len(data.Deps))
			graph.mu.Lock()
			max := data.Block.Round
			round := graph.round
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
					pmax, exists := graph.max[dep.Prev]
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
					lmax, exists := graph.max[link]
					if !exists {
						rcheck = true
					} else if lmax > dmax {
						dmax = lmax
					}
				}
				if rcheck && round > dmax {
					dmax = round
				}
				graph.max[dep.Block] = dmax
				if dmax > max {
					max = dmax
				}
			}
			rcheck := false
			if data.Block.Round != 1 {
				pmax, exists := graph.max[data.Prev]
				if !exists {
					rcheck = true
				} else if pmax > max {
					max = pmax
				}
			}
			if rcheck && round > max {
				max = round
			}
			graph.max[data.Block] = max
			graph.blocks = append(graph.blocks, &blockInfo{
				data: data,
				max:  max,
			})
			graph.mu.Unlock()
			for _, e := range entries {
				graph.process(e)
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
			graph.process(self)
		case <-graph.ctx.Done():
			return
		}
	}
}

// Add updates the graph and notifies the appropriate controllers.
func (graph *Graph) Add(data *BlockGraph) {
	if log.AtDebug() {
		log.Debug("Adding block to graph", fld.BlockID(data.Block))
	}
	graph.entries <- data
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
