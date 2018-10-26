package node // import "chainspace.io/prototype/node"

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base32"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"chainspace.io/prototype/broadcast"
	"chainspace.io/prototype/checker"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/contracts"
	"chainspace.io/prototype/internal/crypto/signature"
	"chainspace.io/prototype/internal/freeport"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/kv"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/pubsub"
	"chainspace.io/prototype/restsrv"
	"chainspace.io/prototype/sbac"
	"chainspace.io/prototype/service"
	"github.com/gogo/protobuf/proto"
	"github.com/tav/golly/process"
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
	Contracts   *config.Contracts
}

// Server represents a running Chainspace node.
type Server struct {
	Broadcast       *broadcast.Service
	cancel          context.CancelFunc
	checker         *checker.Service
	ctx             context.Context
	id              uint64
	key             signature.KeyPair
	keys            map[uint64]signature.PublicKey
	readTimeout     time.Duration
	maxPayload      int
	mu              sync.RWMutex // protects nonceMap
	nonceExpiration time.Duration
	nonceMap        map[uint64][]usedNonce
	restsrv         *restsrv.Service
	sharder         service.Handler
	top             *network.Topology
	sbac            service.Handler
	writeTimeout    time.Duration
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	c := network.NewConn(conn)
	hello, err := c.ReadHello(s.maxPayload, s.readTimeout)
	if err != nil {
		if network.AbnormalError(err) {
			log.Error("Unable to read hello message", fld.Err(err))
		}
		return
	}
	var (
		svc    service.Handler
		peerID uint64
	)
	switch hello.Type {
	case service.CONNECTION_BROADCAST:
		svc = s.Broadcast
		peerID, err = s.verifyPeerID(hello)
		if err != nil {
			log.Error("Unable to verify peer ID from the hello message", fld.Err(err))
			return
		}
	case service.CONNECTION_SBAC:
		svc = s.sbac
		peerID, err = s.verifyPeerID(hello)
		if err != nil {
			log.Error("Unable to verify peer ID from the hello message", fld.Err(err))
			return
		}
	case service.CONNECTION_CHECKER:
		svc = s.checker
		peerID, err = s.verifyPeerID(hello)
		if err != nil {
			log.Error("Unable to verify peer ID from the hello message", fld.Err(err))
			return
		}
	default:
		log.Error("Unknown connection type", fld.ConnectionType(int32(hello.Type)))
		return
	}
	for {
		msg, err := c.ReadMessage(s.maxPayload, s.readTimeout)

		if err != nil {
			if network.AbnormalError(err) {
				log.Error("Could not decode message from an incoming stream", fld.Err(err))
			}
			return
		}

		resp, err := svc.Handle(peerID, msg)
		if err != nil {
			// log.Error("Received error response", fld.Service(svc.Name()), fld.Err(err))
			conn.Close()
			return
		}
		if resp != nil {
			if err = c.WritePayload(resp, s.maxPayload, s.writeTimeout); err != nil {
				if network.AbnormalError(err) {
					log.Error("Unable to write response to peer", fld.PeerID(peerID), fld.Err(err))
				}
				return
			}
		}
	}

}

func (s *Server) listen(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			s.cancel()
			log.Fatal("Could not accept new connections", fld.Err(err))
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

// Shutdown closes all underlying resources associated with the node.
func (s *Server) Shutdown() {
	s.cancel()
}

func createDir(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		log.Info("Creating directory", fld.Path(path))
		if err = os.MkdirAll(path, dirPerms); err != nil {
			return err
		}
	}
	return nil
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

	if err := createDir(dir); err != nil {
		return nil, err
	}
	if err := process.Init(dir, "chainspace"); err != nil {
		return nil, err
	}

	if cfg.Node.Logging.FileLevel >= log.DebugLevel && cfg.Node.Logging.FilePath != "" {
		logfile := filepath.Join(dir, cfg.Node.Logging.FilePath)
		if err := createDir(filepath.Dir(logfile)); err != nil {
			return nil, err
		}
		if err := log.ToFile(logfile, cfg.Node.Logging.FileLevel); err != nil {
			log.Fatal("Could not initialise the file logger", fld.Err(err))
		}
	}

	// start the different contracts
	cts, err := contracts.New(cfg.Contracts)
	if err != nil {
		log.Fatal("unable to instantiate contacts", fld.Err(err))
	}

	// initialize the contracts
	if cfg.Node.Contracts.Manage {
		err = cts.Start()
		if err != nil {
			log.Fatal("unable to start contracts", fld.Err(err))
		}
	}

	// ensure all the contracts are working
	if err := cts.EnsureUp(); err != nil {
		log.Fatal("some contracts are unavailable", fld.Err(err))
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

	// Bootstrap via network registries.
	if cfg.Node.Bootstrap.Registry {
		top.BootstrapRegistries(cfg.Node.Registries)
	}

	// Bootstrap using a static map of addresses.
	if cfg.Node.Bootstrap.Static != nil {
		if err = top.BootstrapStatic(cfg.Node.Bootstrap.Static); err != nil {
			return nil, err
		}
	}

	// Get a port to listen on.
	port, err := freeport.TCP("")
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

	log.SetGlobal(fld.SelfNodeID(cfg.NodeID), fld.SelfShardID(shardID))

	idx := 0
	peers := make([]uint64, len(nodes)-1)
	for _, peer := range nodes {
		if peer != cfg.NodeID {
			peers[idx] = peer
			idx++
		}
	}

	maxPayload, err := cfg.Network.MaxPayload.Int()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	bcfg := &broadcast.Config{
		Broadcast:     cfg.Node.Broadcast,
		Connections:   cfg.Node.Connections,
		Directory:     dir,
		Key:           key,
		Keys:          keys,
		MaxPayload:    maxPayload,
		NetConsensus:  cfg.Network.Consensus,
		NodeConsensus: cfg.Node.Consensus,
		NodeID:        cfg.NodeID,
		Peers:         peers,
	}

	broadcaster, err := broadcast.New(ctx, bcfg, top)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("node: unable to instantiate the broadcast service: %s", err)
	}

	var (
		kvstore *kv.Service
		rstsrv  *restsrv.Service
		ssbac   *sbac.Service
		pbsb    *pubsub.Server
		checkr  *checker.Service
	)

	if cfg.Node.Pubsub.Enabled {
		cfg := &pubsub.Config{
			Port:      cfg.Node.Pubsub.Port,
			NetworkID: cfg.NetworkName,
			NodeID:    cfg.NodeID,
		}
		pbsb, err = pubsub.New(cfg)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("node: unable to instantiate the pubsub server %v", err)
		}
	}

	tcheckers := []checker.Checker{}
	checkers := cts.GetCheckers()
	for _, v := range checkers {
		tcheckers = append(tcheckers, v)
	}

	if !cfg.Node.DisableSBAC {
		checkercfg := &checker.Config{
			Checkers:   tcheckers,
			SigningKey: cfg.Keys.SigningKey,
		}
		checkr, err = checker.New(checkercfg)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("node unable to instantiate checker service: %v", err)
		}

		kvcfg := &kv.Config{
			RuntimeDir: dir,
		}
		kvstore, err = kv.New(kvcfg)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("node: unable to instantiate the kv service: %v", err)
		}
		tcfg := &sbac.Config{
			Broadcaster: broadcaster,
			KVStore:     kvstore,
			Directory:   dir,
			Key:         key,
			MaxPayload:  maxPayload,
			NodeID:      cfg.NodeID,
			ShardCount:  uint64(cfg.Network.Shard.Count),
			ShardSize:   uint64(cfg.Network.Shard.Size),
			SigningKey:  cfg.Keys.SigningKey,
			Top:         top,
			Pubsub:      pbsb,
		}
		ssbac, err = sbac.New(tcfg)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("node: unable to instantiate the sbac service: %s", err)
		}
		if cfg.Node.HTTP.Enabled {
			var rport int
			if cfg.Node.HTTP.Port > 0 {
				rport = cfg.Node.HTTP.Port
			} else {
				rport, _ = freeport.TCP("")
			}
			restsrvcfg := &restsrv.Config{
				Addr:       "",
				Key:        key,
				Port:       rport,
				Top:        top,
				SelfID:     cfg.NodeID,
				MaxPayload: config.ByteSize(maxPayload),
				SBAC:       ssbac,
				Store:      kvstore,
			}
			rstsrv = restsrv.New(restsrvcfg)
		}
	}

	node := &Server{
		Broadcast:       broadcaster,
		cancel:          cancel,
		checker:         checkr,
		ctx:             ctx,
		id:              cfg.NodeID,
		key:             key,
		keys:            keys,
		maxPayload:      maxPayload,
		nonceExpiration: cfg.Network.Consensus.NonceExpiration,
		nonceMap:        map[uint64][]usedNonce{},
		readTimeout:     cfg.Node.Connections.ReadTimeout,
		restsrv:         rstsrv,
		top:             top,
		sbac:            ssbac,
		writeTimeout:    cfg.Node.Connections.WriteTimeout,
	}

	// Start listening on the given port.
	l, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), tlsConf)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("node: unable to listen on port %d: %s", port, err)
	}

	go node.pruneNonceMap()
	go node.listen(l)

	// Announce our location to the world.
	if cfg.Node.Announce.MDNS {
		if err = announceMDNS(cfg.Network.ID, cfg.NodeID, port); err != nil {
			return nil, err
		}
	}
	if cfg.Node.Announce.Registry {
		if err := announceRegistry(cfg.Node.Registries, cfg.Network.ID, cfg.NodeID, port); err != nil {
			return nil, err
		}
	}

	log.Info("Node is running", fld.NetworkName(cfg.NetworkName), fld.Port(port))
	log.Info("Runtime directory", fld.Path(dir))
	return node, nil

}
