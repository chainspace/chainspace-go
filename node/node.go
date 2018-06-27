package node // import "chainspace.io/prototype/node"

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/lucas-clemente/quic-go"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/freeport"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"chainspace.io/prototype/service/broadcast"
	"chainspace.io/prototype/state"
	"chainspace.io/prototype/storage"
	"chainspace.io/prototype/storage/memstore"
	"github.com/tav/golly/log"
)

const (
	initialBackoff = 2 * time.Second
	dirPerms       = 0700
	maxBackoff     = 30 * time.Second
	maxMessage     = 1 << 27 // 128MB
	readTimeout    = 30 * time.Second
	writeTimeout   = 30 * time.Second
)

// Error values.
var (
	ErrDirectoryMissing = errors.New("node: config is missing a value for Directory")
)

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
	broadcaster  *broadcast.Service
	cancel       context.CancelFunc
	db           storage.DB
	ctx          context.Context
	id           uint64
	readTimeout  time.Duration
	sharder      service.Handler
	top          *network.Topology
	transactor   service.Handler
	writeTimeout time.Duration
}

func (s *Server) handleConnection(conn quic.Session) {
	for {
		stream, err := conn.AcceptStream()
		if err != nil {
			log.Errorf("Unable to open new stream from session: %s", err)
			return
		}
		go s.handleStream(stream)
	}
}

func (s *Server) handleStream(stream quic.Stream) {
	defer stream.Close()
	buf := make([]byte, 1)
	if _, err := stream.Read(buf); err != nil {
		log.Errorf("Unable to read stream type: %s", err)
		return
	}
	var svc service.Handler
	switch buf[0] {
	case 1:
		svc = s.broadcaster
	case 2:
		svc = s.transactor
	default:
		log.Errorf("Unknown stream type: %#v", buf[0])
		if err := stream.Close(); err != nil {
			log.Errorf("Received error on attempt to close stream: %s", err)
		}
		return
	}
	ctx := stream.Context()
	for {
		msg, err := s.readMessage(stream)
		if err != nil {
			log.Errorf("Could not decode message from an incoming stream: %s", err)
			return
		}
		resp, err := svc.Handle(ctx, msg)
		if err != nil {
			log.Errorf("Received error response from the %s service: %s", svc.Name(), err)
			return
		}
		if resp != nil {
			// send back response
		}
	}

}

func (s *Server) listen(l quic.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			s.cancel()
			log.Fatalf("node: could not accept new connections: %s", err)
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) maintainBroadcast(ctx context.Context, peerID uint64) {
	backoff := initialBackoff
	for {
		select {
		case <-s.ctx.Done():
			break
		default:
		}
		conn, err := s.top.Dial(ctx, peerID)
		if err == nil {
			backoff = initialBackoff
		} else {
			log.Errorf("Couldn't dial %d: %s", peerID, err)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			time.Sleep(backoff)
			continue
		}
		// conn.SendHello()
		// for msg := range db.NextBlock(peerID) {
		// 	select {
		// 	case <-s.ctx.Done():
		// 		break
		// 	default:
		// 	}
		// 	if err := conn.SendMessage(msg); err != nil {
		// 		break
		// 	}
		// }
		// log.Infof("Connected to node %d", peerID)
		// _, err = conn.Write([]byte("hello"))
		// if err != nil {
		// 	log.Infof("Failed to write on connection to node %d: %s", peerID, err)
		// }
		_ = conn
		time.Sleep(60 * time.Second)
	}
}

func (s *Server) readMessage(stream quic.Stream) (*service.Message, error) {
	buf := make([]byte, 4)
	need := 4
	for need > 0 {
		stream.SetReadDeadline(time.Now().Add(s.readTimeout))
		n, err := stream.Read(buf[4-need:])
		if err != nil {
			log.Errorf("Got error reading from stream: %s", err)
			return nil, err
		}
		need -= n
	}
	// TODO(tav): Fix for 32-bit systems.
	size := int(binary.LittleEndian.Uint32(buf))
	if size > maxMessage {
		log.Errorf("Message size of %d exceeds the max message size", size)
		return nil, fmt.Errorf("node: message size %d exceeds max message size", size)
	}
	buf = make([]byte, size)
	need = size
	for need > 0 {
		stream.SetReadDeadline(time.Now().Add(s.readTimeout))
		n, err := stream.Read(buf[size-need:])
		if err != nil {
			log.Errorf("Got error reading from stream: %s", err)
			return nil, err
		}
		need -= n
	}
	msg := &service.Message{}
	err := proto.Unmarshal(buf, msg)
	return msg, err
}

// AddTransaction adds the provided raw transaction data to the node's
// blockchain.
// func (s *Server) AddTransaction(txdata *TransactionData) {
// 	s.transactor.AddTransaction(txdata)
// }

// Shutdown closes all underlying connections associated with the node.
func (s *Server) Shutdown() {
	s.cancel()
}

// Run initialises a node with the given config.
func Run(cfg *Config) (*Server, error) {

	var err error

	// Initialise the runtime directory for the node.
	dir := cfg.Directory
	if dir == "" {
		return nil, ErrDirectoryMissing
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
		log.Infof("Creating %s", dir)
		if err = os.MkdirAll(dir, dirPerms); err != nil {
			return nil, err
		}
	}

	// Initialise the topology.
	top, err := network.New(cfg.NetworkName, cfg.Network)
	if err != nil {
		return nil, err
	}

	// Bootstrap using a contacts JSON file.
	if cfg.Node.BootstrapFile != "" {
		if err = top.BootstrapFile(cfg.Node.BootstrapFile); err != nil {
			return nil, err
		}
	}

	// Bootstrap using mDNS.
	if cfg.Node.BootstrapMDNS {
		top.BootstrapMDNS()
	}

	// Bootstrap using a static map of addresses.
	if cfg.Node.BootstrapStatic != nil {
		if err = top.BootstrapStatic(cfg.Node.BootstrapStatic); err != nil {
			return nil, err
		}
	}

	// Bootstrap using a URL endpoint.
	if cfg.Node.BootstrapURL != "" {
		if err = top.BootstrapURL(cfg.Node.BootstrapURL); err != nil {
			return nil, err
		}
	}

	// Get a port to listen on.
	port, err := freeport.UDP("")
	if err != nil {
		return nil, err
	}

	// Initialise the TLS cert
	cert, err := tls.X509KeyPair([]byte(cfg.Keys.TransportCert.Public), []byte(cfg.Keys.TransportCert.Private))
	if err != nil {
		return nil, fmt.Errorf("node: could not load the X.509 transport.cert from config: %s", err)
	}
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Initialise the datastore
	var db storage.DB

	switch cfg.Node.Storage.Type {
	case "memstore":
		db = memstore.New()
	case "filestore":
		//
	default:
		return nil, fmt.Errorf("node: unknown storage type: %q", cfg.Node.Storage.Type)
	}

	state := state.Load(db)
	broadcaster := broadcast.New(&broadcast.Config{NodeID: cfg.NodeID}, top, state)

	ctx, cancel := context.WithCancel(context.Background())
	node := &Server{
		broadcaster: broadcaster,
		cancel:      cancel,
		ctx:         ctx,
		db:          db,
		id:          cfg.NodeID,
		top:         top,
	}

	if cfg.Node.ConnectionReadTimeout == 0 {
		node.readTimeout = readTimeout
	} else {
		node.readTimeout = cfg.Node.ConnectionReadTimeout
	}

	if cfg.Node.ConnectionWriteTimeout == 0 {
		node.writeTimeout = writeTimeout
	} else {
		node.writeTimeout = cfg.Node.ConnectionWriteTimeout
	}

	// Maintain persistent connections to all other nodes in our shard.
	shardID := top.ShardForNode(cfg.NodeID)
	peers := top.NodesInShard(shardID)
	for _, peerID := range peers {
		if peerID != cfg.NodeID {
			go node.maintainBroadcast(node.ctx, peerID)
		}
	}

	// Start listening on the given port.
	l, err := quic.ListenAddr(fmt.Sprintf(":%d", port), tlsConf, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("node: unable to listen on port %d: %s", port, err)
	}

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

	log.Infof("Node %d of the %s network is now running on port %d", cfg.NodeID, cfg.NetworkName, port)
	log.Infof("Runtime directory: %s", dir)
	return node, nil

}
