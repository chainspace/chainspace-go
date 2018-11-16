// Package broadcast implements the network broadcast and consensus within a
// shard.
package broadcast // import "chainspace.io/prototype/broadcast"

import (
	"context"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"chainspace.io/prototype/blockmania"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/internal/crypto/signature"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"github.com/dgraph-io/badger"
	"github.com/gogo/protobuf/proto"
	"github.com/tav/golly/process"
)

var errTransactionTooLarge = errors.New("broadcast: transaction is too large")

// Callback defines the delivery callback registered with the broadcast service.
// The callback should call Acknowledge at some point so as to mark the delivery
// of the round as successful.
type Callback func(round uint64, blocks []*SignedData)

// Config for the broadcast service.
type Config struct {
	Broadcast     *config.Broadcast
	Connections   *config.Connections
	Directory     string
	Key           signature.KeyPair
	Keys          map[uint64]signature.PublicKey
	MaxPayload    int
	NetConsensus  *config.NetConsensus
	NodeConsensus *config.NodeConsensus
	NodeID        uint64
	Peers         []uint64
}

type Broadcaster interface {
	AddTransaction(txdata []byte, fee uint64) error
	Register(cb Callback)
}

// Service implements the broadcast and consensus system.
//
// TODO(tav): pad/re-order to minimise false sharing.
type Service struct {
	cb           Callback
	cfg          *Config
	ctx          context.Context
	depgraph     *depgraph
	graph        *blockmania.Graph
	imu          sync.Mutex // protects cb, interpreted, toDeliver
	interpreted  uint64
	key          signature.KeyPair
	keys         map[uint64]signature.PublicKey
	maxBlocks    int
	maxPayload   int
	mu           sync.RWMutex // protects previous, round, sent, signal, txCountLimit
	nodeID       uint64
	ownblocks    *ownblocks
	peers        []uint64
	prevhash     []byte
	prevref      *SignedData
	readTimeout  time.Duration
	refSizeLimit int
	received     *receivedMap
	round        uint64
	sent         map[uint64]uint64
	signal       chan struct{}
	store        *store
	toDeliver    []*blockmania.Interpreted
	top          *network.Topology
	txCount      int
	txCountLimit int
	txData       []byte
	txIdx        int
	txSizeLimit  int
	txmu         sync.Mutex // protects txCount, txData, txIdx
	writeTimeout time.Duration
}

func (s *Service) byzcoCallback(data *blockmania.Interpreted) {
	if log.AtInfo() {
		blocks := make([]string, len(data.Blocks))
		for i, block := range data.Blocks {
			blocks[i] = block.String()
		}
		log.Info("Got interpreted consensus blocks", fld.Round(data.Round),
			log.Uint64("consumed", data.Consumed), log.Strings("blocks", blocks), log.Int("block.count", len(data.Blocks)))
	}
	s.store.setInterpreted(data)
	s.imu.Lock()
	if s.cb == nil {
		s.interpreted = data.Round
		s.imu.Unlock()
		return
	}
	s.imu.Unlock()
	s.cb(data.Round, s.getBlocks(data.Blocks))
}

func (s *Service) fillMissingBlocks(peerID uint64) {
	var (
		latest uint64
		rounds []uint64
	)
	initialBackoff := s.cfg.Broadcast.InitialBackoff
	interval := s.cfg.NetConsensus.RoundInterval
	backoff := initialBackoff
	getRounds := &GetRounds{}
	getRoundsReq := &service.Message{Opcode: int32(OP_GET_ROUNDS)}
	list := &ListBlocks{}
	l := log.With(fld.PeerID(peerID))
	maxBackoff := s.cfg.Broadcast.MaxBackoff
	retry := false
	for {
		if retry {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			time.Sleep(backoff)
			retry = false
		}
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		conn, err := s.top.Dial(peerID, s.readTimeout)
		if err == nil {
			backoff = initialBackoff
		} else {
			retry = true
			continue
		}
		hello, err := service.SignHello(s.nodeID, peerID, s.key, service.CONNECTION_BROADCAST)
		if err != nil {
			l.Error("Couldn't create Hello payload for filling missing blocks", fld.Err(err))
			retry = true
			continue
		}
		if err = conn.WritePayload(hello, s.maxPayload, s.writeTimeout); err != nil {
			if network.AbnormalError(err) {
				l.Error("Couldn't send Hello", fld.Err(err))
			}
			retry = true
			continue
		}
		for {
			select {
			case <-s.ctx.Done():
				return
			default:
			}
			if rounds == nil {
				info := s.received.get(peerID)
				if info.latest == info.sequence {
					time.Sleep(interval)
					continue
				}
				rounds, latest, err = s.store.getMissing(peerID, info.sequence, s.maxBlocks)
				if err != nil {
					l.Fatal("Could not load missing rounds info from DB", fld.Err(err))
				}
				if len(rounds) == 0 {
					if latest != info.sequence && latest != 0 {
						s.received.setSequence(peerID, latest)
					}
					continue
				}
				if log.AtDebug() {
					l.Debug("Got missing rounds info from DB", fld.Rounds(rounds), fld.LatestRound(latest))
				}
			}
			getRounds.Rounds = rounds
			getRoundsReq.Payload, err = proto.Marshal(getRounds)
			if err != nil {
				l.Fatal("Could not encode request for get rounds", fld.Err(err))
			}
			if err = conn.WritePayload(getRoundsReq, s.maxPayload, s.writeTimeout); err != nil {
				if network.AbnormalError(err) {
					l.Error("Could not write get rounds request", fld.Err(err))
				}
				retry = true
				break
			}
			resp, err := conn.ReadMessage(s.maxPayload, s.readTimeout)
			if err != nil {
				if network.AbnormalError(err) {
					l.Error("Could not read response to get rounds request", fld.Err(err))
				}
				retry = true
				break
			}
			list.Blocks = nil
			if err := proto.Unmarshal(resp.Payload, list); err != nil {
				l.Error("Could not decode response to get rounds request", fld.Err(err))
				retry = true
				break
			}
			if len(list.Blocks) == 0 {
				l.Error("Received empty map of hashes")
				retry = true
				break
			}
			seen := map[uint64]struct{}{}
			for _, block := range list.Blocks {
				round, err := s.processBlock(block)
				if err != nil {
					l.Error("Could not process block received in response to get rounds request", fld.Err(err))
					continue
				}
				seen[round] = struct{}{}
			}
			var latestSeen uint64
			var missing []uint64
			for _, round := range rounds {
				if _, exists := seen[round]; !exists {
					missing = append(missing, round)
					continue
				}
				latestSeen = round
			}
			if latestSeen != 0 {
				if len(missing) != 0 {
					s.received.setSequence(peerID, latest)
				} else {
					s.received.setSequence(peerID, latestSeen)
				}
			}
			if len(missing) == 0 {
				rounds = nil
			} else {
				rounds = missing
			}
		}
	}
}

func (s *Service) genBlocks() {
	block := &Block{
		Node:         s.nodeID,
		Transactions: &Transactions{},
	}
	driftTolerance := s.cfg.NodeConsensus.DriftTolerance
	hasher := sha512.New512_256()
	initialRate := s.cfg.NodeConsensus.RateLimit.InitialRate
	interval := s.cfg.NetConsensus.RoundInterval
	overloaded := false
	rate := initialRate
	rateDecr := s.cfg.NodeConsensus.RateLimit.RateDecrease
	rateIncr := s.cfg.NodeConsensus.RateLimit.RateIncrease
	rootRef := &BlockReference{
		Node: s.nodeID,
	}
	round := s.round
	start := time.Now().Truncate(interval)
	workDuration := s.cfg.NodeConsensus.InitialWorkDuration
	next := start.Add(interval)
	workStart := next.Add(-workDuration)
	time.Sleep(workStart.Sub(time.Now().Round(0)))
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		if time.Since(workStart) > driftTolerance {
			rate = int(float64(rate) * rateDecr)
			if rate < initialRate {
				rate = initialRate
			}
			s.mu.Lock()
			s.txCountLimit = rate
			s.mu.Unlock()
			if !overloaded {
				if log.AtDebug() {
					log.Debug("Overloaded")
				}
				overloaded = true
			}
		} else {
			rate += rateIncr
			s.mu.Lock()
			s.txCountLimit = rate
			s.mu.Unlock()
			if overloaded {
				if log.AtDebug() {
					log.Debug("Not overloaded")
				}
				overloaded = false
			}
		}
		round++
		refs := s.depgraph.getBlocks(s.refSizeLimit)
		blocks := make([]*SignedData, len(refs))
		for i, info := range refs {
			if log.AtDebug() {
				log.Debug("Including block", fld.BlockID(info.id))
			}
			blocks[i] = info.ref
		}
		block.Previous = s.prevref
		block.References = blocks
		block.Round = round
		s.txmu.Lock()
		txcount := s.txCount
		block.Transactions.Count = uint64(txcount)
		block.Transactions.Data = s.txData[:s.txIdx]
		data, err := proto.Marshal(block)
		if err != nil {
			log.Fatal("Got unexpected error encoding latest block", log.Err(err))
		}
		s.txCount = 0
		s.txIdx = 0
		s.txmu.Unlock()
		log.Info("Creating block", fld.Round(round), log.Int("txs", txcount), log.Int("blocks", len(blocks)))
		if _, err := hasher.Write(data); err != nil {
			log.Fatal("Could not hash encoded block", log.Err(err))
		}
		hash := hasher.Sum(nil)
		rootRef.Hash = hash
		rootRef.Round = round
		refData, err := proto.Marshal(rootRef)
		if err != nil {
			log.Fatal("Got unexpected error encoding latest block reference", log.Err(err))
		}
		signed := &SignedData{
			Data:      data,
			Signature: s.key.Sign(refData),
		}
		blockRef := &SignedData{
			Data:      refData,
			Signature: signed.Signature,
		}
		hasher.Reset()
		signal := s.signal
		graph := &blockmania.BlockGraph{
			Block: blockmania.BlockID{
				Hash:  string(hash),
				Node:  s.nodeID,
				Round: round,
			},
		}
		if round != 1 {
			graph.Prev = blockmania.BlockID{
				Hash:  string(s.prevhash),
				Node:  s.nodeID,
				Round: round - 1,
			}
		}
		var deps []blockmania.Dep
		for _, ref := range refs {
			deps = append(deps, blockmania.Dep{
				Block: ref.id,
				Deps:  ref.deps,
				Prev:  ref.prev,
			})
		}
		graph.Deps = deps
		if err := s.setOwnBlock(signed, blockRef, graph); err != nil {
			log.Fatal("Could not write own block to the DB", fld.Round(round), log.Err(err))
		}
		for _, ref := range refs {
			s.depgraph.actuallyIncluded(ref.id)
		}
		s.graph.Add(graph)
		s.prevhash = hash
		s.prevref = &SignedData{
			Data:      refData,
			Signature: signed.Signature,
		}
		s.mu.Lock()
		s.round = round
		s.signal = make(chan struct{})
		s.mu.Unlock()
		close(signal)
		if log.AtDebug() {
			log.Debug("Created block", fld.Round(block.Round))
		}
		if round%50 == 0 {
			s.ownblocks.prune(50)
		}
		now := time.Now().Round(0)
		workDuration = now.Sub(workStart)
		next = next.Add(interval)
		workStart = next.Add(-workDuration)
		time.Sleep(workStart.Sub(now))
	}
}

func (s *Service) getBlock(nodeID uint64, round uint64, hash []byte) *SignedData {
	block, err := s.store.getBlock(blockmania.BlockID{
		Hash:  string(hash),
		Node:  nodeID,
		Round: round,
	})
	if err != nil {
		log.Error("Could not retrieve block", fld.NodeID(nodeID), fld.Round(round), log.Err(err))
		return nil
	}
	return block
}

func (s *Service) getBlocks(ids []blockmania.BlockID) []*SignedData {
	blocks, err := s.store.getBlocks(ids)
	if err != nil {
		log.Fatal("Unable to retrieve blocks", log.Err(err))
	}
	return blocks
}

func (s *Service) getOwnBlock(round uint64) (*SignedData, error) {
	block := s.ownblocks.get(round)
	if block != nil {
		return block, nil
	}
	block, err := s.store.getOwnBlock(round)
	if err != nil {
		return nil, err
	}
	s.ownblocks.set(round, block)
	return block, nil
}

// getUnsent returns a slice of unsent blocks for the given peer. If it looks
// like the peer is up-to-date, then the call blocks until a new block is
// available.
func (s *Service) getUnsent(peerID uint64) []*SignedData {
	for {
		s.mu.RLock()
		cur := s.round
		last := s.sent[peerID] + 1
		if cur >= last {
			blocks := []*SignedData{}
			total := 0
			for i := last; i <= cur; i++ {
				block, err := s.getOwnBlock(i)
				if err != nil {
					log.Fatal("Unable to retrieve own block", fld.Round(i), log.Err(err))
				}
				total += len(block.Data) + len(block.Signature) + 100
				if total > s.maxPayload {
					if i == last {
						log.Fatal("Size of individual block exceeds max payload size", fld.Size(total), fld.PayloadLimit(s.maxPayload))
					}
					s.mu.RUnlock()
					return blocks
				}
				blocks = append(blocks, block)
			}
			s.mu.RUnlock()
			return blocks
		}
		signal := s.signal
		s.mu.RUnlock()
		select {
		case <-signal:
		case <-s.ctx.Done():
			return nil
		}
	}
}

func (s *Service) handleBroadcastList(peerID uint64, msg *service.Message) (*service.Message, error) {
	list := &ListBlocks{}
	if err := proto.Unmarshal(msg.Payload, list); err != nil {
		return nil, err
	}
	var last uint64
	for _, block := range list.Blocks {
		round, err := s.processBlock(block)
		if err != nil {
			log.Error("Could not process block received in broadcast", fld.PeerID(peerID), fld.Err(err))
			return nil, err
		}
		last = round
	}
	data, err := proto.Marshal(&AckBroadcast{
		Last: last,
	})
	if err != nil {
		return nil, err
	}
	return &service.Message{
		Opcode:  int32(OP_ACK_BROADCAST),
		Payload: data,
	}, nil
}

func (s *Service) handleGetBlocks(peerID uint64, msg *service.Message) (*service.Message, error) {
	req := &GetBlocks{}
	if err := proto.Unmarshal(msg.Payload, req); err != nil {
		return nil, err
	}
	if len(req.Blocks) > s.maxBlocks {
		return nil, fmt.Errorf("broadcast: requested %d blocks exceeds max block limit of %d", len(req.Blocks), s.maxBlocks)
	}
	blocks := make([]*SignedData, len(req.Blocks))
	for i, ref := range req.Blocks {
		block := s.getBlock(ref.Node, ref.Round, ref.Hash)
		if block == nil {
			blocks[i] = nil
			log.Error("Got request for unknown block", fld.PeerID(peerID), fld.Round(ref.Round))
			continue
		}
		blocks[i] = block
	}
	data, err := proto.Marshal(&ListBlocks{
		Blocks: blocks,
	})
	if err != nil {
		return nil, err
	}
	return &service.Message{
		Opcode:  int32(OP_LIST_BLOCKS),
		Payload: data,
	}, nil
}

func (s *Service) handleGetRounds(peerID uint64, msg *service.Message) (*service.Message, error) {
	req := &GetRounds{}
	if err := proto.Unmarshal(msg.Payload, req); err != nil {
		return nil, err
	}
	if len(req.Rounds) > s.maxBlocks {
		return nil, fmt.Errorf("broadcast: requested %d rounds exceeds max block limit of %d", len(req.Rounds), s.maxBlocks)
	}
	var blocks []*SignedData
	for _, round := range req.Rounds {
		block, err := s.getOwnBlock(round)
		if err != nil {
			log.Error("Unable to retrieve own block for get rounds request", fld.Round(round), log.Err(err))
			continue
		}
		blocks = append(blocks, block)
	}
	data, err := proto.Marshal(&ListBlocks{
		Blocks: blocks,
	})
	if err != nil {
		return nil, err
	}
	return &service.Message{
		Opcode:  int32(OP_LIST_BLOCKS),
		Payload: data,
	}, nil
}

func (s *Service) loadState() {
	round, prevhash, prevref, err := s.store.getLastRoundData()
	if err != nil {
		if err != badger.ErrKeyNotFound {
			log.Fatal("Could not load latest round data from DB", log.Err(err))
		}
		prevhash = nil
		prevref = nil
		round = 0
	}
	interpreted, consumed, err := s.store.getLastInterpreted()
	if err != nil {
		if err != badger.ErrKeyNotFound {
			log.Fatal("Could not load last interpreted from DB", log.Err(err))
		}
		consumed = 0
		interpreted = 0
	}
	rmap, err := s.store.getReceivedMap()
	if err != nil {
		if err != badger.ErrKeyNotFound {
			log.Fatal("Could not load received map from DB", log.Err(err))
		}
		rmap = map[uint64]receivedInfo{}
		for _, peer := range s.peers {
			rmap[peer] = receivedInfo{}
		}
	}
	sent, err := s.store.getSentMap()
	if err != nil {
		if err != badger.ErrKeyNotFound {
			log.Fatal("Could not load sent map from DB", log.Err(err))
		}
		sent = map[uint64]uint64{}
		for _, peer := range s.peers {
			sent[peer] = 0
		}
	}
	if log.AtDebug() {
		log.Debug("Startup state", fld.Round(round), fld.InterpretedRound(interpreted))
	}
	nodes := append([]uint64{s.nodeID}, s.peers...)
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i] < nodes[j]
	})
	gcfg := &blockmania.Config{
		LastInterpreted: interpreted,
		Nodes:           nodes,
		SelfID:          s.nodeID,
		TotalNodes:      s.top.TotalNodes(),
	}
	depgraph := &depgraph{
		await:   map[blockmania.BlockID][]blockmania.BlockID{},
		cond:    sync.NewCond(&sync.Mutex{}),
		ctx:     s.ctx,
		icache:  map[blockmania.BlockID]bool{},
		pending: map[blockmania.BlockID]*blockData{},
		self:    s.nodeID,
		store:   s.store,
		tcache:  map[blockmania.BlockID]bool{},
	}
	s.depgraph = depgraph
	s.graph = blockmania.New(s.ctx, gcfg, s.byzcoCallback)
	s.interpreted = interpreted
	s.prevhash = prevhash
	s.prevref = prevref
	s.received = &receivedMap{data: rmap}
	s.round = round
	s.sent = sent
	s.signal = make(chan struct{})
	s.replayGraphChanges(consumed)
	go depgraph.process()
}

func (s *Service) maintainBroadcast(peerID uint64) {
	ack := &AckBroadcast{}

	initialBackoff := s.cfg.Broadcast.InitialBackoff
	interval := s.cfg.NetConsensus.RoundInterval
	backoff := initialBackoff
	maxBackoff := s.cfg.Broadcast.MaxBackoff
	msg := &service.Message{Opcode: int32(OP_BROADCAST)}
	list := &ListBlocks{}
	retry := false
	for {
		if retry {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			time.Sleep(backoff)
			retry = false
		}
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		conn, err := s.top.Dial(peerID, s.readTimeout)
		if err == nil {
			backoff = initialBackoff
		} else {
			retry = true
			continue
		}
		hello, err := service.SignHello(s.nodeID, peerID, s.key, service.CONNECTION_BROADCAST)
		if err != nil {
			log.Error("Couldn't create Hello payload for broadcast", log.Err(err))
			retry = true
			continue
		}
		if err = conn.WritePayload(hello, s.maxPayload, s.writeTimeout); err != nil {
			if network.AbnormalError(err) {
				log.Error("Couldn't send Hello", fld.PeerID(peerID), log.Err(err))
			}
			retry = true
			continue
		}
		for {
			select {
			case <-s.ctx.Done():
				return
			default:
			}
			blocks := s.getUnsent(peerID)
			if blocks == nil {
				return
			}
			if len(blocks) == 0 {
				time.Sleep(interval)
				continue
			}
			list.Blocks = blocks
			msg.Payload, err = proto.Marshal(list)
			if err != nil {
				log.Fatal("Could not encode list blocks for broadcast", log.Err(err))
			}
			if err = conn.WritePayload(msg, s.maxPayload, s.writeTimeout); err != nil {
				if network.AbnormalError(err) {
					log.Error("Could not write list blocks", fld.PeerID(peerID), log.Err(err))
				}
				retry = true
				break
			}
			resp, err := conn.ReadMessage(s.maxPayload, s.readTimeout)
			if err != nil {
				if network.AbnormalError(err) {
					log.Error("Could not read broadcast ack response", fld.PeerID(peerID), log.Err(err))
				}
				retry = true
				break
			}
			if err = proto.Unmarshal(resp.Payload, ack); err != nil {
				log.Error("Could not decode broadcast ack response", fld.PeerID(peerID), log.Err(err))
				retry = true
				break
			}
			s.markSent(peerID, ack.Last)
		}
	}
}

// markSent marks the given round as being sent to the peer in the shard.
func (s *Service) markSent(peerID uint64, round uint64) {
	s.mu.Lock()
	s.sent[peerID] = round
	s.mu.Unlock()
}

func (s *Service) processBlock(signed *SignedData) (uint64, error) {
	block := &Block{}
	if err := proto.Unmarshal(signed.Data, block); err != nil {
		return 0, err
	}
	key, exists := s.keys[block.Node]
	if !exists {
		return 0, fmt.Errorf("broadcast: unable to find signing.key for node %d", block.Node)
	}
	hasher := sha512.New512_256()
	if _, err := hasher.Write(signed.Data); err != nil {
		return 0, fmt.Errorf("broadcast: unable to hash received signed block data: %s", err)
	}
	hash := hasher.Sum(nil)
	ref := &BlockReference{
		Hash:  hash,
		Node:  block.Node,
		Round: block.Round,
	}
	enc, err := proto.Marshal(ref)
	if err != nil {
		return 0, fmt.Errorf("broadcast: unable to encode block reference: %s", err)
	}
	if !key.Verify(enc, signed.Signature) {
		return 0, fmt.Errorf("broadcast: unable to verify signature for block %d from node %d", block.Round, block.Node)
	}
	var prevID blockmania.BlockID
	prev := &BlockReference{}
	if block.Round != 1 {
		if block.Previous == nil {
			return 0, fmt.Errorf("broadcast: nil previous block on block %d from node %d", block.Round, block.Node)
		}
		if !key.Verify(block.Previous.Data, block.Previous.Signature) {
			return 0, fmt.Errorf("broadcast: unable to verify signature for previous block on block %d from node %d", block.Round, block.Node)
		}
		if err = proto.Unmarshal(block.Previous.Data, prev); err != nil {
			return 0, err
		}
		if prev.Node != block.Node {
			return 0, fmt.Errorf("broadcast: previous block node does not match on block %d from node %d", block.Round, block.Node)
		}
		if prev.Round+1 != block.Round {
			return 0, fmt.Errorf("broadcast: previous block round is invalid on block %d from node %d", block.Round, block.Node)
		}
		prevID = blockmania.BlockID{
			Hash:  string(prev.Hash),
			Node:  prev.Node,
			Round: prev.Round,
		}
	}
	var deps []blockmania.BlockID
	for i, sref := range block.References {
		ref := &BlockReference{}
		if err := proto.Unmarshal(sref.Data, ref); err != nil {
			return 0, fmt.Errorf("broadcast: unable to decode block reference at position %d for block %d from node %d", i, block.Round, block.Node)
		}
		key, exists := s.keys[ref.Node]
		if !exists {
			return 0, fmt.Errorf("broadcast: unable to find signing.key for node %d in block reference %d for block %d from node %d", ref.Node, i, block.Round, block.Node)
		}
		if !key.Verify(sref.Data, sref.Signature) {
			return 0, fmt.Errorf("broadcast: unable to verify signature for block reference %d for block %d from node %d", i, block.Round, block.Node)
		}
		if ref.Node == block.Node {
			return 0, fmt.Errorf("broadcast: block references includes a block from same node in block reference %d for block %d from node %d", i, block.Round, block.Node)
		}
		deps = append(deps, blockmania.BlockID{
			Hash:  string(ref.Hash),
			Node:  ref.Node,
			Round: ref.Round,
		})
	}
	if log.AtDebug() {
		log.Debug("Received block", fld.NodeID(block.Node), fld.Round(block.Round))
	}
	id := blockmania.BlockID{
		Hash:  string(hash),
		Node:  block.Node,
		Round: block.Round,
	}
	if err := s.setBlock(id, signed); err != nil {
		log.Fatal("Could not set received block", fld.BlockID(id), fld.Err(err))
	}
	s.depgraph.add(&blockData{
		deps: deps,
		id:   id,
		prev: prevID,
		ref: &SignedData{
			Data:      enc,
			Signature: signed.Signature,
		},
	})
	s.received.set(block.Node, block.Round)
	return block.Round, nil
}

func (s *Service) replayGraphChanges(from uint64) {
	blocks, err := s.store.getNotIncluded()
	if err != nil {
		log.Fatal("Unable to load block data for unincluded blocks", fld.Err(err))
	}
	for _, block := range blocks {
		if log.AtDebug() {
			log.Debug("Adding to depgraph", fld.BlockID(block.id))
		}
		s.depgraph.add(block)
	}
	const batchSize = 100
	for {
		blocks, err := s.store.getBlockGraphs(s.nodeID, from, batchSize)
		if err != nil {
			log.Fatal("Unable to load block graphs for uninterpreted blocks", fld.Err(err))
		}
		for _, block := range blocks {
			s.graph.Add(block)
		}
		if len(blocks) < batchSize {
			break
		}
		from += batchSize
	}
}

func (s *Service) setBlock(id blockmania.BlockID, block *SignedData) error {
	return s.store.setBlock(id, block)
}

func (s *Service) setOwnBlock(block *SignedData, blockRef *SignedData, graph *blockmania.BlockGraph) error {
	s.ownblocks.set(graph.Block.Round, block)
	return s.store.setOwnBlock(block, blockRef, graph)
}

func (s *Service) writeReceivedMap() {
	ticker := time.NewTicker(5 * s.cfg.NetConsensus.RoundInterval)
	for {
		select {
		case <-ticker.C:
			rmap := make(map[uint64]receivedInfo, len(s.peers))
			s.received.mu.RLock()
			for k, v := range s.received.data {
				rmap[k] = v
			}
			s.received.mu.RUnlock()
			s.store.setReceivedMap(rmap)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Service) writeSentMap() {
	ticker := time.NewTicker(5 * s.cfg.NetConsensus.RoundInterval)
	for {
		select {
		case <-ticker.C:
			sent := make(map[uint64]uint64, len(s.peers))
			s.mu.RLock()
			for k, v := range s.sent {
				sent[k] = v
			}
			s.mu.RUnlock()
			s.store.setSentMap(sent)
		case <-s.ctx.Done():
			return
		}
	}
}

// Acknowledge should be called by BroadcastEnd in the registered Callback to
// mark a particular round as being fully delivered. If not, all unacknowledged
// rounds will be replayed when the node restarts.
func (s *Service) Acknowledge(round uint64) {
	s.store.setDeliverAcknowledged(round)
}

// AddTransaction adds the given transaction data onto a queue to be added to
// the current block.
func (s *Service) AddTransaction(txdata []byte, fee uint64) error {
	size := len(txdata)
	if size >= (1<<32) || size > s.txSizeLimit {
		return errTransactionTooLarge
	}
	for {
		s.mu.RLock()
		overloaded := s.txCount == s.txCountLimit
		signal := s.signal
		s.mu.RUnlock()
		if overloaded {
			<-signal
			continue
		}
		s.txmu.Lock()
		if s.txIdx+size+100 > s.txSizeLimit {
			s.txmu.Unlock()
			<-signal
			continue
		}
		binary.LittleEndian.PutUint64(s.txData[s.txIdx:s.txIdx+8], fee)
		binary.LittleEndian.PutUint32(s.txData[s.txIdx+8:s.txIdx+12], uint32(size))
		copy(s.txData[s.txIdx+12:], txdata)
		s.txCount++
		s.txIdx += 12 + size
		s.txmu.Unlock()
		return nil
	}
}

// Handle implements the service Handler interface for handling messages
// received over a connection.
func (s *Service) Handle(peerID uint64, msg *service.Message) (*service.Message, error) {
	switch OP(msg.Opcode) {
	case OP_BROADCAST:
		return s.handleBroadcastList(peerID, msg)
	case OP_GET_BLOCKS:
		return s.handleGetBlocks(peerID, msg)
	case OP_GET_ROUNDS:
		return s.handleGetRounds(peerID, msg)
	default:
		return nil, fmt.Errorf("broadcast: unknown message opcode: %d", msg.Opcode)
	}
}

// Name specifies the name of the service for use in debugging service handlers.
func (s *Service) Name() string {
	return "broadcast"
}

// Register saves the given callback to be called when transactions have reached
// consensus. It'll also trigger the replaying of any unacknowledged rounds.
func (s *Service) Register(cb Callback) {
	s.imu.Lock()
	if s.cb != nil {
		s.mu.Unlock()
		log.Fatal("Attempt to register a broadcast.Callback when one already exists")
	}
	s.cb = cb
	ack, err := s.store.getDeliverAcknowledged()
	if err != nil {
		if err != badger.ErrKeyNotFound {
			log.Fatal("Could not load latest acknowledged round from DB", log.Err(err))
		}
	}
	latest := s.interpreted
	for i := ack + 1; i <= latest; i++ {
		blocks, err := s.store.getInterpreted(i)
		if err != nil {
			log.Fatal("Unable to load blocks for a round", fld.Round(i), log.Err(err))
		}
		cb(i, blocks)
	}
	s.imu.Unlock()
}

// New returns a fully instantiated broadcaster service.
func New(ctx context.Context, cfg *Config, top *network.Topology) (*Service, error) {
	opts := badger.DefaultOptions
	opts.Dir = filepath.Join(cfg.Directory, "broadcast")
	opts.ValueDir = opts.Dir
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	ownblocks := &ownblocks{
		data: map[uint64]*SignedData{},
	}
	store := &store{
		db:        db,
		nodeCount: len(cfg.Peers) + 1,
	}
	process.SetExitHandler(func() {
		store.mu.Lock()
		defer store.mu.Unlock()
		if store.closed {
			return
		}
		log.Info("Closing DB")
		store.closed = true
		if err := db.Close(); err != nil {
			log.Error("Could not close the broadcast DB successfully", fld.Err(err))
		}
	})
	refSizeLimit, err := cfg.NetConsensus.BlockReferencesSizeLimit.Int()
	if err != nil {
		return nil, err
	}
	txSizeLimit, err := cfg.NetConsensus.BlockTransactionsSizeLimit.Int()
	if err != nil {
		return nil, err
	}
	s := &Service{
		cfg:          cfg,
		ctx:          ctx,
		key:          cfg.Key,
		keys:         cfg.Keys,
		maxBlocks:    cfg.MaxPayload / (refSizeLimit + txSizeLimit + 100),
		maxPayload:   cfg.MaxPayload,
		nodeID:       cfg.NodeID,
		ownblocks:    ownblocks,
		peers:        cfg.Peers,
		readTimeout:  cfg.Connections.ReadTimeout,
		refSizeLimit: refSizeLimit,
		store:        store,
		top:          top,
		txCountLimit: cfg.NodeConsensus.RateLimit.InitialRate,
		txData:       make([]byte, txSizeLimit),
		txSizeLimit:  txSizeLimit,
		writeTimeout: cfg.Connections.WriteTimeout,
	}
	s.loadState()
	go s.genBlocks()
	go s.writeReceivedMap()
	go s.writeSentMap()
	for _, peer := range cfg.Peers {
		go s.fillMissingBlocks(peer)
		go s.maintainBroadcast(peer)
	}
	return s, nil
}
