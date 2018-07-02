// Package broadcast implements the network broadcast and consensus within a
// shard.
package broadcast

import (
	"context"
	"fmt"
	"sync"
	"time"

	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"github.com/gogo/protobuf/proto"
	"github.com/tav/golly/log"
)

var (
	genesis = []byte("genesis.block")
)

// Callback defines the interface for the callback set registered with the
// broadcast service. The BroadcastStart method is called as soon as a
// particular round reaches consensus, followed by individual
// BroadcastTransaction calls for each of the transactions in consensus order,
// and finished off with a BroadcastEnd call for each round. The BroadcastEnd
// method should call Acknowledge at some point so as to mark the delivery of
// the round as successful.
type Callback interface {
	BroadcastStart(round uint64)
	BroadcastTransaction(txdata *TransactionData, block *Block)
	BroadcastEnd(round uint64)
}

// Config for the broadcast service.
type Config struct {
	ConsensusInterval time.Duration
	Directory         string
	Key               signature.KeyPair
	Keys              map[uint64]signature.PublicKey
	NodeID            uint64
	Peers             []uint64
}

// Service implements the broadcast and consensus system.
type Service struct {
	blocks   map[uint64]*SignedBlock
	cb       Callback
	cond     *sync.Cond
	ctx      context.Context
	dir      string
	entries  chan *Entry
	interval time.Duration
	key      signature.KeyPair
	keys     map[uint64]signature.PublicKey
	mu       sync.RWMutex // protects blocks, previous, round, sent
	nodeID   uint64
	peers    []uint64
	previous []byte
	round    uint64
	sent     map[uint64]uint64
	top      *network.Topology
}

func (s *Service) genBlocks() {
	entries := []*Entry{}
	// TODO(tav): time.Ticker's behaviour around slow receivers may not be the
	// exact semantics we want.
	tick := time.NewTicker(s.interval)
	s.mu.Lock()
	round := s.round
	s.mu.Unlock()
	for {
		select {
		case entry := <-s.entries:
			entries = append(entries, entry)
		case <-tick.C:
			round++
			block := &Block{
				Entries:  entries,
				Node:     s.nodeID,
				Number:   round,
				Previous: s.previous,
			}
			data, err := proto.Marshal(block)
			if err != nil {
				log.Fatalf("Got unexpected error encoding block: %s", err)
			}
			signed := &SignedBlock{
				Data:      data,
				Signature: s.key.Sign(data),
			}
			s.previous = signed.Digest()
			// if err := s.db.Sync(); err != nil {
			// 	log.Fatalf("Got unexpected error when syncing the DB: %s", err)
			// }
			s.mu.Lock()
			s.blocks[block.Number] = signed
			s.round = round
			s.mu.Unlock()
			s.cond.Broadcast()
			log.Infof("Created new block: %#v", signed)
		case <-s.ctx.Done():
			tick.Stop()
			return
		}
	}
}

func (s *Service) handleBlock(peerID uint64, msg *service.Message) error {
	signed := &SignedBlock{}
	if err := proto.Unmarshal(msg.Payload, signed); err != nil {
		return err
	}
	return s.processBlock(signed)
}

func (s *Service) handleBlocksResponse(peerID uint64, msg *service.Message) error {
	resp := &BlocksResponse{}
	if err := proto.Unmarshal(msg.Payload, resp); err != nil {
		return err
	}
	for _, signed := range resp.Signed {
		if err := s.processBlock(signed); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) loadState() {
	// TODO(tav): Load these from a filestore-backed DB.
	s.blocks = map[uint64]*SignedBlock{}
	s.previous = genesis
	s.round = 0
	s.sent = map[uint64]uint64{}
	for _, peer := range s.peers {
		s.sent[peer] = 0
	}
}

func (s *Service) processBlock(signed *SignedBlock) error {
	block := &Block{}
	if err := proto.Unmarshal(signed.Data, block); err != nil {
		return err
	}
	key, exists := s.keys[block.Node]
	if !exists {
		return fmt.Errorf("broadcast: unable to find signing.key for node %d", block.Node)
	}
	if !key.Verify(signed.Data, signed.Signature) {
		return fmt.Errorf("broadcast: unable to verify signature for block %d from node %d", block.Number, block.Node)
	}
	return nil
}

// Acknowledge should be called by BroadcastEnd in the registered Callback to
// mark a particular round as being fully delivered. If not, all unacknowledged
// rounds will be replayed when the node restarts.
func (s *Service) Acknowledge(round uint64) {
}

// AddTransaction adds the given transaction data onto a queue to be added to
// the current block.
func (s *Service) AddTransaction(txdata *TransactionData) {
	s.entries <- &Entry{Value: &Entry_Transaction{txdata}}
}

// Handle implements the service Handler interface for handling messages
// received over a connection.
func (s *Service) Handle(ctx context.Context, peerID uint64, msg *service.Message) (*service.Message, error) {
	switch OP(msg.Opcode) {
	case OP_BLOCK:
		return nil, s.handleBlock(peerID, msg)
	case OP_BLOCKS_REQUEST:
		return nil, nil
	case OP_BLOCKS_RESPONSE:
		return nil, s.handleBlocksResponse(peerID, msg)
	default:
		return nil, fmt.Errorf("broadcast: unknown message opcode: %d", msg.Opcode)
	}
}

// MarkSent marks blocks for the given round as being sent to the peer in the
// shard.
func (s *Service) MarkSent(peerID uint64, round uint64) error {
	s.mu.Lock()
	last := s.sent[peerID]
	if round != last+1 {
		return fmt.Errorf("broadcast: MarkSent called with round %d for node %d, when last round sent was only %d", round, peerID, last)
	}
	s.sent[peerID] = round
	s.mu.Unlock()
	return nil
}

// Name specifies the name of the service for use in debugging service handlers.
func (s *Service) Name() string {
	return "broadcast"
}

// Register saves the given callback to be called when transactions have reached
// consensus.
func (s *Service) Register(cb Callback) {
	s.cb = cb
}

// New returns a fully instantiated broadcaster service. Soon after the service
// is instantiated, Register should be called on it to register a callback,
// before any AddTransaction calls are made.
func New(ctx context.Context, cfg *Config, top *network.Topology) *Service {
	s := &Service{
		ctx:      ctx,
		dir:      cfg.Directory,
		entries:  make(chan *Entry, 10000),
		interval: cfg.ConsensusInterval,
		key:      cfg.Key,
		keys:     cfg.Keys,
		nodeID:   cfg.NodeID,
		peers:    cfg.Peers,
		top:      top,
	}
	s.cond = sync.NewCond(&s.mu)
	s.loadState()
	go s.genBlocks()
	return s
}
