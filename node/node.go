package node // import "chainspace.io/prototype/node"

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base32"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/freeport"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"chainspace.io/prototype/service/broadcast"
	"chainspace.io/prototype/service/transactor"
	"github.com/gogo/protobuf/proto"
	"github.com/lucas-clemente/quic-go"
	"github.com/tav/golly/process"
	"go.uber.org/zap"
)

const (
	dirPerms = 0700
)

// Error values.
var (
	errDirectoryMissing = errors.New("node: config is missing a value for Directory")
	errInvalidSignature = errors.New("node: invalid signature on hello")
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

type usedNonce struct {
	nonce []byte
	ts    time.Time
}

// Config represents the configuration of an individual node within a Chainspace network.
type Config struct {
	Directory   string
	Keys        *config.Keys
	Network     *config.Network
	NetworkName string
	NodeID      uint64
	Node        *config.Node
}

// Server represents a running Chainspace node.
type Server struct {
	broadcaster     *broadcast.Service
	cancel          context.CancelFunc
	ctx             context.Context
	id              uint64
	key             signature.KeyPair
	keys            map[uint64]signature.PublicKey
	readTimeout     time.Duration
	maxPayload      int
	mu              sync.RWMutex // protects nonceMap
	nonceExpiration time.Duration
	nonceMap        map[uint64][]usedNonce
	sharder         service.Handler
	top             *network.Topology
	transactor      service.Handler
	writeTimeout    time.Duration
}

func (s *Server) handleConnection(conn quic.Session) {
	for {
		stream, err := conn.AcceptStream()
		if err != nil {
			log.Error("Unable to open new stream from session", zap.Error(err))
			return
		}
		go s.handleStream(stream)
	}
}

func (s *Server) handleStream(stream quic.Stream) {
	defer stream.Close()
	c := network.NewConn(stream)
	hello, err := c.ReadHello(s.maxPayload, s.readTimeout)
	if err != nil {
		log.Error("Unable to read hello message from stream", zap.Error(err))
		return
	}
	var (
		svc    service.Handler
		peerID uint64
	)
	switch hello.Type {
	case service.CONNECTION_BROADCAST:
		svc = s.broadcaster
		peerID, err = s.verifyPeerID(hello)
		if err != nil {
			log.Error("Unable to verify peer ID from the hello message", zap.Error(err))
			return
		}
	case service.CONNECTION_TRANSACTOR:
		svc = s.transactor
		peerID, err = s.verifyPeerID(hello)
		if err != nil {
			log.Error("Unable to verify peer ID from the hello message", zap.Error(err))
			return
		}
		log.Info("New transactor hello nesssage")
	default:
		log.Error("Unknown connection type", zap.Int32("type", int32(hello.Type)))
		return
	}
	ctx := stream.Context()
	for {
		msg, err := c.ReadMessage(s.maxPayload, s.readTimeout)
		if err != nil {
			log.Error("Could not decode message from an incoming stream", zap.Error(err))
			return
		}
		resp, err := svc.Handle(ctx, peerID, msg)
		if err != nil {
			log.Error("Received error response", zap.String("service", svc.Name()), zap.Error(err))
			return
		}
		if resp != nil {
			if err = c.WritePayload(resp, s.maxPayload, s.writeTimeout); err != nil {
				log.Error("Unable to write response to peer", zap.Uint64("peer.id", peerID), zap.Error(err))
				return
			}
		}
	}

}

func (s *Server) listen(l quic.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			s.cancel()
			log.Fatal("Could not accept new connections", zap.Error(err))
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) pruneNonceMap() {
	exp := s.nonceExpiration
	tick := time.NewTicker(exp)
	var xs []usedNonce
	for {
		select {
		case <-tick.C:
			now := time.Now()
			nmap := map[uint64][]usedNonce{}
			s.mu.Lock()
			for nodeID, nonces := range s.nonceMap {
				xs = nil
				for _, used := range nonces {
					diff := now.Sub(used.ts)
					if diff < 0 {
						diff = -diff
					}
					if diff < exp {
						xs = append(xs, used)
					}
				}
				if xs != nil {
					nmap[nodeID] = xs
				}
			}
			s.nonceMap = nmap
			s.mu.Unlock()
		case <-s.ctx.Done():
			tick.Stop()
			return
		}
	}
}

func (s *Server) verifyPeerID(hello *service.Hello) (uint64, error) {
	if len(hello.Payload) == 0 {
		return 0, nil
	}
	info := &service.HelloInfo{}
	if err := proto.Unmarshal(hello.Payload, info); err != nil {
		return 0, err
	}
	if info.Server != s.id {
		return 0, fmt.Errorf("node: mismatched server field in hello payload: expected %d, got %d", s.id, info.Server)
	}
	key, exists := s.keys[info.Client]
	if !exists {
		return 0, fmt.Errorf("node: could not find public signing key for node %d", info.Client)
	}
	if !key.Verify(hello.Payload, hello.Signature) {
		return 0, fmt.Errorf("node: could not validate signature claiming to be from node %d", info.Client)
	}
	diff := time.Now().Sub(info.Timestamp)
	if diff < 0 {
		diff = -diff
	}
	if diff > s.nonceExpiration {
		return 0, fmt.Errorf("node: timestamp in client hello is outside of the max clock skew range: %d", diff)
	}
	s.mu.RLock()
	nonces, exists := s.nonceMap[info.Client]
	if exists {
		for _, used := range nonces {
			if bytes.Equal(used.nonce, info.Nonce) {
				s.mu.RUnlock()
				return 0, fmt.Errorf("node: received duplicate nonce from %d", info.Client)
			}
		}
	}
	s.mu.RUnlock()
	return info.Client, nil
}

// AddTransaction adds the provided raw transaction data to the node's
// blockchain.
// func (s *Server) AddTransaction(txdata *TransactionData) {
// 	s.transactor.AddTransaction(txdata)
// }

// AddTransactionData is a temporary method for testing. It adds the given
// transaction to the node's current block.
func (s *Server) AddTransactionData(txdata *broadcast.TransactionData) {
	s.broadcaster.AddTransaction(txdata)
}

// Shutdown closes all underlying resources associated with the node.
func (s *Server) Shutdown() {
	s.cancel()
}

// Run initialises a node with the given config.
func Run(cfg *Config) (*Server, error) {
	var err error

	// Initialise the runtime directory for the node.
	dir := cfg.Directory
	if dir == "" {
		return nil, errDirectoryMissing
	}
	if !filepath.IsAbs(dir) {
		if dir, err = filepath.Abs(dir); err != nil {
			return nil, err
		}
	}
	_, err = os.Stat(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		log.Info("Creating directory", zap.String("path", dir))
		if err = os.MkdirAll(dir, dirPerms); err != nil {
			return nil, err
		}
	}

	if err := process.Init(dir, "chainspace"); err != nil {
		return nil, err
	}

	if err := log.InitFileLogger(filepath.Join(dir, "server.log"), log.DebugLevel); err != nil {
		log.Fatal("Could not initialise the file logger", zap.Error(err))
	}

	// Initialise the topology.
	top, err := network.New(cfg.NetworkName, cfg.Network)
	if err != nil {
		return nil, err
	}

	// Bootstrap using a contacts JSON file.
	if cfg.Node.Bootstrap.File != "" {
		if err = top.BootstrapFile(cfg.Node.Bootstrap.File); err != nil {
			return nil, err
		}
	}

	// Bootstrap using mDNS.
	if cfg.Node.Bootstrap.MDNS {
		top.BootstrapMDNS()
	}

	// Bootstrap using a static map of addresses.
	if cfg.Node.Bootstrap.Static != nil {
		if err = top.BootstrapStatic(cfg.Node.Bootstrap.Static); err != nil {
			return nil, err
		}
	}

	// Bootstrap using a URL endpoint.
	if cfg.Node.Bootstrap.URL != "" {
		if err = top.BootstrapURL(cfg.Node.Bootstrap.URL); err != nil {
			return nil, err
		}
	}

	// Get a port to listen on.
	port, err := freeport.UDP("")
	if err != nil {
		return nil, err
	}

	// Initialise the TLS cert.
	cert, err := tls.X509KeyPair([]byte(cfg.Keys.TransportCert.Public), []byte(cfg.Keys.TransportCert.Private))
	if err != nil {
		return nil, fmt.Errorf("node: could not load the X.509 transport.cert from config: %s", err)
	}
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Initialise signing keys.
	var key signature.KeyPair
	switch cfg.Keys.SigningKey.Type {
	case "ed25519":
		pubkey, err := b32.DecodeString(cfg.Keys.SigningKey.Public)
		if err != nil {
			return nil, fmt.Errorf("node: could not decode the base32-encoded public signing.key from config: %s", err)
		}
		privkey, err := b32.DecodeString(cfg.Keys.SigningKey.Private)
		if err != nil {
			return nil, fmt.Errorf("node: could not decode the base32-encoded private signing.key from config: %s", err)
		}
		key, err = signature.LoadKeyPair(signature.Ed25519, append(pubkey, privkey...))
		if err != nil {
			return nil, fmt.Errorf("node: unable to load the signing.key from config: %s", err)
		}
	default:
		return nil, fmt.Errorf("node: unknown type of signing.key found in config: %q", cfg.Keys.SigningKey.Type)
	}

	keys := top.SeedPublicKeys()
	shardID := top.ShardForNode(cfg.NodeID)
	nodes := top.NodesInShard(shardID)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("node: got no peers for the node %d", cfg.NodeID)
	}

	log.SetGlobalFields(zap.Uint64("self.node.id", cfg.NodeID), zap.Uint64("self.shard.id", shardID))

	idx := 0
	peers := make([]uint64, len(nodes)-1)
	for _, peer := range nodes {
		if peer != cfg.NodeID {
			peers[idx] = peer
			idx++
		}
	}

	blockLimit, err := cfg.Network.Consensus.BlockLimit.Int()
	if err != nil {
		return nil, err
	}

	maxPayload, err := cfg.Network.MaxPayload.Int()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	bcfg := &broadcast.Config{
		BlockLimit:     blockLimit,
		Directory:      dir,
		Key:            key,
		Keys:           keys,
		InitialBackoff: cfg.Node.Broadcast.InitialBackoff,
		MaxBackoff:     cfg.Node.Broadcast.MaxBackoff,
		MaxPayload:     maxPayload,
		NodeID:         cfg.NodeID,
		Peers:          peers,
		ReadTimeout:    cfg.Node.Connections.ReadTimeout,
		RoundInterval:  cfg.Network.Consensus.RoundInterval,
		WriteTimeout:   cfg.Node.Connections.WriteTimeout,
	}

	broadcaster, err := broadcast.New(ctx, bcfg, top)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("node: unable to instantiate the broadcast service: %s", err)
	}

	tcfg := &transactor.Config{
		Broadcaster: broadcaster,
		Checkers: []transactor.Checker{
			&transactor.DummyCheckerOK{}, &transactor.DummyCheckerKO{}},
		Directory:  dir,
		Key:        key,
		MaxPayload: maxPayload,
		NodeID:     cfg.NodeID,
		ShardCount: uint64(cfg.Network.Shard.Count),
		ShardSize:  uint64(cfg.Network.Shard.Size),
		SigningKey: cfg.Keys.SigningKey,
		Top:        top,
	}

	txtor, err := transactor.New(tcfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("node: unable to instantiate the transactor service: %s", err)
	}

	node := &Server{
		broadcaster:     broadcaster,
		cancel:          cancel,
		ctx:             ctx,
		id:              cfg.NodeID,
		key:             key,
		keys:            keys,
		maxPayload:      maxPayload,
		nonceExpiration: cfg.Network.Consensus.NonceExpiration,
		nonceMap:        map[uint64][]usedNonce{},
		readTimeout:     cfg.Node.Connections.ReadTimeout,
		top:             top,
		transactor:      txtor,
		writeTimeout:    cfg.Node.Connections.WriteTimeout,
	}

	// Start listening on the given port.
	l, err := quic.ListenAddr(fmt.Sprintf(":%d", port), tlsConf, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("node: unable to listen on port %d: %s", port, err)
	}

	go node.pruneNonceMap()
	go node.listen(l)

	// Announce our location to the world.
	for _, meth := range cfg.Node.Announce {
		if meth == "mdns" {
			if err = announceMDNS(cfg.Network.ID, cfg.NodeID, port); err != nil {
				return nil, err
			}
		} else if strings.HasPrefix(meth, "https://") || strings.HasPrefix(meth, "http://") {
			// Announce to the given URL endpoint.
		} else {
			return nil, fmt.Errorf("node: announcement endpoint %q does not start with http:// or https://", meth)
		}
	}

	log.Info("Node is running", zap.String("network.name", cfg.NetworkName), zap.Int("port", port))
	log.Info("Runtime directory", zap.String("path", dir))
	return node, nil

}
