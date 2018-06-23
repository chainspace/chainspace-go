package node // import "chainspace.io/prototype/node"

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lucas-clemente/quic-go"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/freeport"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/storage"
	"chainspace.io/prototype/storage/memstore"
	"github.com/tav/golly/log"
)

const (
	dirPerms = 0700
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
	ctx context.Context
	top *network.Topology
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
		if err = top.BootstrapMDNS(); err != nil {
			return nil, err
		}
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
	default:
		return nil, fmt.Errorf("node: unknown storage type: %q", cfg.Node.Storage.Type)
	}

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

	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{
		ctx: ctx,
		top: top,
	}

	// Maintain persistent connections to all other nodes in our shard.
	shardID := top.ShardForNode(cfg.NodeID)
	peers := top.NodesInShard(shardID)
	for _, peerID := range peers {
		if peerID != cfg.NodeID {
			go server.maintainConnection(cfg.NodeID, peerID, db)
		}
	}

	// Start listening on the given port.
	l, err := quic.ListenAddr(fmt.Sprintf(":%d", port), tlsConf, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("node: unable to listen on port %d: %s", port, err)
	}

	go server.listen(l, cancel)

	log.Infof("Node %d of the %s network is now running on port %d", cfg.NodeID, cfg.NetworkName, port)
	log.Infof("Runtime directory: %s", dir)
	return server, nil

}

const (
	initialBackoff = 2 * time.Second
	maxBackoff     = 30 * time.Second
)

// State represents the internal state of a node.
type State struct {
	db storage.DB
}

func (s *Server) maintainConnection(nodeID uint64, peerID uint64, db storage.DB) {
	backoff := initialBackoff
	for {
		conn, err := s.top.Dial(peerID)
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
		select {
		case <-s.ctx.Done():
			break
		default:
		}
		log.Infof("Connected to node %d", peerID)
		_, err = conn.Write([]byte("hello"))
		if err != nil {
			log.Infof("Failed to write on connection to node %d: %s", peerID, err)
		}
		time.Sleep(60 * time.Second)
	}
}

func (s *Server) listen(l quic.Listener, cancel context.CancelFunc) {
	for {
		conn, err := l.Accept()
		if err != nil {
			cancel()
			log.Fatalf("node: could not accept new connections: %s", err)
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn quic.Session) {
	stream, err := conn.AcceptStream()
	if err != nil {
		log.Errorf("Unable to open new stream from session: %s", err)
		return
	}
	buf := make([]byte, 5)
	n, err := stream.Read(buf)
	if err != nil {
		log.Errorf("Unable to read incoming stream: %s", err)
		return
	}
	log.Infof("Received %q from connection", string(buf[:n]))
}
