// Package broadcast implements the network broadcast and consensus within a
// shard.
package broadcast // import "chainspace.io/prototype/broadcast"

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"chainspace.io/prototype/byzco"
	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"github.com/dgraph-io/badger"
	"github.com/gogo/protobuf/proto"
)

var (
	genesis = []byte("genesis.block")
)

// Callback defines the interface for the callback set registered with the
// broadcast service. The DeliverStart method is called as soon as a particular
// round reaches consensus, followed by individual DeliverTransaction calls for
// each of the transactions in consensus order, and finished off with a
// DeliverEnd call for each round. The DeliverEnd method should call Acknowledge
// at some point so as to mark the delivery of the round as successful.
type Callback interface {
	DeliverStart(round uint64)
	DeliverTransaction(txdata *TransactionData)
	DeliverEnd(round uint64)
}

// Config for the broadcast service.
type Config struct {
	BlockLimit     int
	Directory      string
	Key            signature.KeyPair
	Keys           map[uint64]signature.PublicKey
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	MaxPayload     int
	NodeID         uint64
	Peers          []uint64
	ReadTimeout    time.Duration
	RoundInterval  time.Duration
	WriteTimeout   time.Duration
}

// Service implements the broadcast and consensus system.
type Service struct {
	blockLimit     int
	cb             Callback
	cond           *sync.Cond // protects interpreted, toDeliver
	ctx            context.Context
	dag            *byzco.DAG
	depgraph       *depgraph
	initialBackoff time.Duration
	interpreted    uint64
	interval       time.Duration
	key            signature.KeyPair
	keys           map[uint64]signature.PublicKey
	lru            *lru
	maxBackoff     time.Duration
	maxBlocks      int
	maxPayload     int
	mu             sync.RWMutex // protects previous, round, sent, signal
	nodeID         uint64
	ownblocks      *ownblocks
	peers          []uint64
	previous       []byte
	readTimeout    time.Duration
	received       *receivedMap
	refs           chan *blockInfo
	round          uint64
	sent           map[uint64]uint64
	signal         chan struct{}
	store          *store
	toDeliver      []*byzco.Interpreted
	top            *network.Topology
	txs            chan *TransactionData
	writeTimeout   time.Duration
}

// NOTE(tav): toDeliver will keep growing indefinitely if a Callback is never
// registered.
func (s *Service) dagCallback(data *byzco.Interpreted) {
	s.store.setInterpreted(data)
	s.cond.L.Lock()
	s.interpreted = data.Round
	s.toDeliver = append(s.toDeliver, data)
	s.cond.L.Unlock()
	s.cond.Signal()
}

func (s *Service) deliver() {
	s.cond.L.Lock()
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
		s.deliverRound(i, blocks)
	}
	s.cond.L.Unlock()
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		s.cond.L.Lock()
		for len(s.toDeliver) == 0 {
			s.cond.Wait()
		}
		data := s.toDeliver[0]
		s.toDeliver = s.toDeliver[1:]
		s.cond.L.Unlock()
		blocks := s.getBlocks(data.Blocks)
		s.deliverRound(data.Round, blocks)
	}
}

func (s *Service) deliverRound(round uint64, blocks []*SignedData) {
	var txlist transactionList
	for _, block := range blocks {
		txs, err := block.Transactions()
		if err != nil {
			log.Fatal("Unable to decode transactions", fld.Round(round), fld.BlockHash(block.Digest()))
		}
		txlist = append(txlist, txs...)
	}
	txlist.Sort()
	s.cb.DeliverStart(round)
	for _, tx := range txlist {
		s.cb.DeliverTransaction(tx)
	}
	s.cb.DeliverEnd(round)
}

func (s *Service) fillMissingBlocks(peerID uint64) {
	var (
		latest uint64
		rounds []uint64
	)
	backoff := s.initialBackoff
	getRounds := &GetRounds{}
	getRoundsReq := &service.Message{Opcode: uint32(OP_GET_ROUNDS)}
	list := &ListBlocks{}
	log := log.With(fld.PeerID(peerID))
	retry := false
	for {
		if retry {
			backoff *= 2
			if backoff > s.maxBackoff {
				backoff = s.maxBackoff
			}
			time.Sleep(backoff)
			retry = false
		}
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		conn, err := s.top.Dial(s.ctx, peerID)
		if err == nil {
			backoff = s.initialBackoff
		} else {
			log.Error("Couldn't dial node", fld.Err(err))
			retry = true
			continue
		}
		hello, err := service.SignHello(s.nodeID, peerID, s.key, service.CONNECTION_BROADCAST)
		if err != nil {
			log.Error("Couldn't create Hello payload for filling missing blocks", fld.Err(err))
			retry = true
			continue
		}
		if err = conn.WritePayload(hello, s.maxPayload, s.writeTimeout); err != nil {
			log.Error("Couldn't send Hello", fld.Err(err))
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
					time.Sleep(s.interval)
					continue
				}
				rounds, latest, err = s.store.getMissing(peerID, info.sequence, s.maxBlocks)
				log.Debug("GOT MISSING", fld.Rounds(rounds), fld.LatestRound(latest))
				if err != nil {
					log.Fatal("Could not load missing rounds from DB", fld.Err(err))
				}
				if len(rounds) == 0 {
					if latest != info.sequence && latest != 0 {
						s.received.setSequence(peerID, latest)
					}
					continue
				}
			}
			getRounds.Rounds = rounds
			getRoundsReq.Payload, err = proto.Marshal(getRounds)
			if err != nil {
				log.Fatal("Could not encode request for get rounds", fld.Err(err))
			}
			if err = conn.WritePayload(getRoundsReq, s.maxPayload, s.writeTimeout); err != nil {
				log.Error("Could not write get rounds request", fld.Err(err))
				retry = true
				break
			}
			resp, err := conn.ReadMessage(s.maxPayload, s.readTimeout)
			if err != nil {
				log.Error("Could not read response to get rounds request", fld.Err(err))
				retry = true
				break
			}
			list.Blocks = nil
			if err := proto.Unmarshal(resp.Payload, list); err != nil {
				log.Error("Could not decode response to get rounds request", fld.Err(err))
				retry = true
				break
			}
			if len(list.Blocks) == 0 {
				log.Error("Received empty map of hashes")
				retry = true
				break
			}
			seen := map[uint64]struct{}{}
			for _, block := range list.Blocks {
				round, err := s.processBlock(block)
				if err != nil {
					log.Error("Could not process block received in response to get rounds request", fld.Err(err))
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
	var (
		atLimit     bool
		pendingRefs []*blockInfo
		pendingTxs  []*TransactionData
		refs        []*blockInfo
		txs         []*TransactionData
	)
	block := &Block{
		Node: s.nodeID,
	}
	hasher := combihash.New()
	rootRef := &BlockReference{
		Node: s.nodeID,
	}
	round := s.round
	// TODO(tav): time.Ticker's behaviour around slow receivers may not be the
	// exact semantics we want.
	tick := time.NewTicker(s.interval)
	total := 0
	for {
		select {
		case info := <-s.refs:
			if atLimit {
				pendingRefs = append(pendingRefs, info)
			} else {
				total += info.ref.Size()
				if total < s.blockLimit {
					refs = append(refs, info)
				} else {
					atLimit = true
					pendingRefs = append(pendingRefs, info)
				}
			}
		case tx := <-s.txs:
			if atLimit {
				pendingTxs = append(pendingTxs, tx)
			} else {
				total += tx.Size()
				if total < s.blockLimit {
					txs = append(txs, tx)
				} else {
					atLimit = true
					pendingTxs = append(pendingTxs, tx)
				}
			}
		case <-tick.C:
			round++
			blocks := make([]*SignedData, len(refs))
			for i, info := range refs {
				blocks[i] = info.ref
			}
			block.Previous = s.previous
			block.References = blocks
			block.Round = round
			block.Transactions = txs
			data, err := proto.Marshal(block)
			if err != nil {
				log.Fatal("Got unexpected error encoding latest block", log.Err(err))
			}
			if _, err := hasher.Write(data); err != nil {
				log.Fatal("Could not hash encoded block", log.Err(err))
			}
			hash := hasher.Digest()
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
			s.previous = hash
			hasher.Reset()
			signal := s.signal
			if err := s.setOwnBlock(round, hash, signed); err != nil {
				log.Fatal("Could not write own block to the DB", fld.Round(round), log.Err(err))
			}
			if err := s.store.setCurrentRoundAndHash(round, hash); err != nil {
				log.Fatal("Could not write current round and hash to the DB", log.Err(err))
			}
			s.mu.Lock()
			s.round = round
			s.signal = make(chan struct{})
			s.mu.Unlock()
			close(signal)
			// TODO(tav): Remove this once it's fully wired together.
			s.dagCallback(&byzco.Interpreted{
				Blocks: []byzco.BlockID{{Hash: string(hash), NodeID: s.nodeID, Round: round}},
				Round:  round,
			})
			log.Debug("Created block", fld.Round(block.Round))
			if round%100 == 0 {
				s.lru.prune(len(s.peers) * 100)
				s.ownblocks.prune(100)
			}
			txs = nil
			total = 0
			if len(pendingTxs) > 0 {
				var (
					npendingRefs []*blockInfo
					npendingTxs  []*TransactionData
				)
				for _, info := range pendingRefs {
					if len(npendingTxs) > 0 {
						npendingRefs = append(npendingRefs, info)
					} else {
						total += info.ref.Size()
						if total < s.blockLimit {
							refs = append(refs, info)
						} else {
							npendingRefs = append(npendingRefs, info)
						}
					}
				}
				for _, tx := range pendingTxs {
					if len(npendingTxs) > 0 {
						npendingTxs = append(npendingTxs, tx)
					} else {
						total += tx.Size()
						if total < s.blockLimit {
							txs = append(txs, tx)
						} else {
							npendingTxs = append(npendingTxs, tx)
						}
					}
				}
				pendingTxs = npendingTxs
			}
		case <-s.ctx.Done():
			tick.Stop()
			if err := s.store.db.Close(); err != nil {
				log.Error("Could not close the broadcast DB successfully", log.Err(err))
			}
			return
		}
	}
}

func (s *Service) getBlockInfo(nodeID uint64, round uint64, hash []byte) *blockInfo {
	id := byzco.BlockID{
		Hash:   string(hash),
		NodeID: nodeID,
		Round:  round,
	}
	info := s.lru.get(id)
	if info != nil {
		return info
	}
	block, err := s.store.getBlock(id)
	if err != nil {
		log.Error("Could not retrieve block", fld.NodeID(nodeID), fld.Round(round), log.Err(err))
		return nil
	}
	info = &blockInfo{
		block: block,
		hash:  hash,
	}
	s.lru.set(byzco.BlockID{
		Hash:   string(hash),
		NodeID: nodeID,
		Round:  round,
	}, info)
	return info
}

func (s *Service) getBlocks(ids []byzco.BlockID) []*SignedData {
	var missing []byzco.BlockID
	blocks := make([]*SignedData, len(ids))
	for i, id := range ids {
		info := s.lru.get(id)
		if info == nil {
			missing = append(missing, id)
		} else {
			blocks[i] = info.block
		}
	}
	if len(missing) > 0 {
		res, err := s.store.getBlocks(missing)
		if err != nil {
			log.Fatal("Unable to retrieve blocks", log.Err(err))
		}
		used := 0
		for i, block := range blocks {
			if block == nil {
				blocks[i] = res[used]
				used++
			}
		}
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
		Opcode:  uint32(OP_ACK_BROADCAST),
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
		info := s.getBlockInfo(ref.Node, ref.Round, ref.Hash)
		if info == nil {
			blocks[i] = nil
			log.Error("Got request for unknown block", fld.PeerID(peerID), fld.Round(ref.Round))
			continue
		}
		blocks[i] = info.block
	}
	data, err := proto.Marshal(&ListBlocks{
		Blocks: blocks,
	})
	if err != nil {
		return nil, err
	}
	return &service.Message{
		Opcode:  uint32(OP_LIST_BLOCKS),
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
		Opcode:  uint32(OP_LIST_BLOCKS),
		Payload: data,
	}, nil
}

func (s *Service) loadState() {
	round, hash, err := s.store.getCurrentRoundAndHash()
	if err != nil {
		if err != badger.ErrKeyNotFound {
			log.Fatal("Could not load latest round and hash from DB", log.Err(err))
		}
		hash = genesis
		round = 0
	}
	interpreted, err := s.store.getLastInterpreted()
	if err != nil {
		if err != badger.ErrKeyNotFound {
			log.Fatal("Could not load last interpreted from DB", log.Err(err))
		}
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
	log.Debug("STARTUP STATE", fld.Round(round), fld.InterpretedRound(interpreted))
	nodes := append([]uint64{s.nodeID}, s.peers...)
	depgraph := &depgraph{
		ctx:  s.ctx,
		refs: s.refs,
	}
	depgraph.cond = sync.NewCond(&depgraph.mu)
	s.dag = byzco.NewDAG(s.ctx, nodes, interpreted, s.dagCallback)
	s.depgraph = depgraph
	s.interpreted = interpreted
	s.previous = hash
	s.received = &receivedMap{data: rmap}
	s.round = round
	s.sent = sent
	s.signal = make(chan struct{})
	s.replayDAGChanges()
}

func (s *Service) maintainBroadcast(peerID uint64) {
	ack := &AckBroadcast{}
	backoff := s.initialBackoff
	msg := &service.Message{Opcode: uint32(OP_BROADCAST)}
	list := &ListBlocks{}
	retry := false
	for {
		if retry {
			backoff *= 2
			if backoff > s.maxBackoff {
				backoff = s.maxBackoff
			}
			time.Sleep(backoff)
			retry = false
		}
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		conn, err := s.top.Dial(s.ctx, peerID)
		if err == nil {
			backoff = s.initialBackoff
		} else {
			log.Error("Couldn't dial node", fld.PeerID(peerID), log.Err(err))
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
			log.Error("Couldn't send Hello", fld.PeerID(peerID), log.Err(err))
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
				time.Sleep(s.interval)
				continue
			}
			list.Blocks = blocks
			msg.Payload, err = proto.Marshal(list)
			if err != nil {
				log.Fatal("Could not encode list blocks for broadcast", log.Err(err))
			}
			if err = conn.WritePayload(msg, s.maxPayload, s.writeTimeout); err != nil {
				log.Error("Could not write list blocks", fld.PeerID(peerID), log.Err(err))
				retry = true
				break
			}
			resp, err := conn.ReadMessage(s.maxPayload, s.readTimeout)
			if err != nil {
				log.Error("Could not read broadcast ack response", fld.PeerID(peerID), log.Err(err))
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
	hasher := combihash.New()
	if _, err := hasher.Write(signed.Data); err != nil {
		return 0, fmt.Errorf("broadcast: unable to hash received signed block data: %s", err)
	}
	hash := hasher.Digest()
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
	var links []byzco.BlockID
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
		links = append(links, byzco.BlockID{
			Hash:   string(ref.Hash),
			NodeID: ref.Node,
			Round:  ref.Round,
		})
	}
	// TOOD(tav): Check and validate .Previous hash.
	log.Debug("Received block", fld.NodeID(block.Node), fld.Round(block.Round))
	id := byzco.BlockID{
		Hash:   string(hash),
		NodeID: block.Node,
		Round:  block.Round,
	}
	s.received.set(block.Node, block.Round)
	return block.Round, s.setBlock(id, signed)
}

func (s *Service) replayDAGChanges() {
	for round := s.interpreted; round < s.round; round++ {
		// block := s.getOwnBlock(round)
	}
}

func (s *Service) setBlock(id byzco.BlockID, block *SignedData) error {
	if err := s.store.setBlock(id, block); err != nil {
		return err
	}
	s.lru.set(id, &blockInfo{
		block: block,
		hash:  []byte(id.Hash),
	})
	return nil
}

func (s *Service) setOwnBlock(round uint64, hash []byte, block *SignedData) error {
	id := byzco.BlockID{
		Hash:   string(hash),
		NodeID: s.nodeID,
		Round:  round,
	}
	s.lru.set(id, &blockInfo{
		block: block,
		hash:  hash,
	})
	s.ownblocks.set(round, block)
	return s.store.setOwnBlock(id, block)
}

func (s *Service) writeReceivedMap() {
	ticker := time.NewTicker(20 * s.interval)
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
	ticker := time.NewTicker(20 * s.interval)
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
func (s *Service) AddTransaction(txdata *TransactionData) {
	s.txs <- txdata
}

// Handle implements the service Handler interface for handling messages
// received over a connection.
func (s *Service) Handle(ctx context.Context, peerID uint64, msg *service.Message) (*service.Message, error) {
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
	s.mu.Lock()
	if s.cb != nil {
		s.mu.Unlock()
		log.Fatal("Attempt to register a broadcast.Callback when one already exists")
	}
	s.cb = cb
	s.mu.Unlock()
	go s.deliver()
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
	lru := &lru{
		data: map[byzco.BlockID]*blockInfoContainer{},
	}
	ownblocks := &ownblocks{
		data: map[uint64]*SignedData{},
	}
	store := &store{
		db:    db,
		nodes: len(cfg.Peers) + 1,
	}
	s := &Service{
		blockLimit:     cfg.BlockLimit,
		cond:           sync.NewCond(&sync.Mutex{}),
		ctx:            ctx,
		interval:       cfg.RoundInterval,
		key:            cfg.Key,
		keys:           cfg.Keys,
		initialBackoff: cfg.InitialBackoff,
		lru:            lru,
		maxBackoff:     cfg.MaxBackoff,
		maxBlocks:      cfg.MaxPayload / cfg.BlockLimit,
		maxPayload:     cfg.MaxPayload,
		nodeID:         cfg.NodeID,
		ownblocks:      ownblocks,
		peers:          cfg.Peers,
		readTimeout:    cfg.ReadTimeout,
		refs:           make(chan *blockInfo, 10000),
		store:          store,
		top:            top,
		txs:            make(chan *TransactionData, 10000),
		writeTimeout:   cfg.WriteTimeout,
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
