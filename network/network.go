package network // import "chainspace.io/prototype/network"

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base32"
	"errors"
	"fmt"
	"sync"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto/signature"
	"github.com/lucas-clemente/quic-go"
	"github.com/tav/golly/log"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

type contacts struct {
	addr map[uint64]string
	cond map[uint64]*sync.Cond
	mu   sync.RWMutex
}

type nodeConfig struct {
	key signature.PublicKey
	tls *tls.Config
}

// Topology represents a Chainspace network.
type Topology struct {
	contacts   *contacts
	id         string
	mu         sync.RWMutex
	name       string
	nodes      map[uint64]*nodeConfig
	shardCount int
	shardSize  int
}

func (t *Topology) maintainMDNS() {
}

// BootstrapFile will use the JSON file at the given path for the initial set of
// addresses for nodes in the network.
func (t *Topology) BootstrapFile(path string) error {
	log.Infof("Bootstrapping via file %s for the %s network [%s]", path, t.name, t.id)
	return errors.New("network: bootstrapping from a static map is not supported yet")
}

// BootstrapMDNS will try to auto-discover the addresses of initial nodes using
// multicast DNS.
func (t *Topology) BootstrapMDNS() error {
	log.Infof("Bootstrapping via mDNS for the %s network [%s]", t.name, t.id)
	go t.maintainMDNS()
	return nil
}

// BootstrapMap will use the given map of addresses for the initial addresses of
// nodes in the network.
func (t *Topology) BootstrapMap(addresses map[uint64]string) error {
	log.Infof("Bootstrapping via static map for the %s network [%s]", t.name, t.id)
	return errors.New("network: bootstrapping from a static map is not supported yet")
}

// BootstrapURL will use the given URL endpoint to discover the initial
// addresses of nodes in the network.
func (t *Topology) BootstrapURL(endpoint string) error {
	log.Infof("Bootstrapping via URL for the %s network [%s]", t.name, t.id)
	return errors.New("network: bootstrapping from a URL is not supported yet")
}

// Dial opens a connection to a node in the given network. It will block if
// unable to find a routing address for the given node.
func (t *Topology) Dial(id uint64) (*Conn, error) {
	t.mu.RLock()
	cfg, exists := t.nodes[id]
	t.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("network: could not find config for node %d in the %s network", id, t.name)
	}
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("network: could not generate randomness for node connection: %s", err)
	}
	session, err := quic.DialAddr(fmt.Sprintf("localhost:900%d", id), cfg.tls, nil)
	if err != nil {
		return nil, fmt.Errorf("network: could not connect to node %d: %s", id, err)
	}
	stream, err := session.OpenStreamSync()
	if err != nil {
		return nil, fmt.Errorf("network: could not open a stream in the connection to %d: %s", id, err)
	}
	_ = stream
	c := &Conn{
		nonceBase: buf,
		// stream:    stream,
	}
	// go c.run()
	return c, nil
}

// New parses the given network configuration and creates a network topology for
// connecting to nodes.
func New(name string, cfg *config.Network) (*Topology, error) {
	var key signature.PublicKey
	contacts := &contacts{
		addr: map[uint64]string{},
		cond: map[uint64]*sync.Cond{},
	}
	cond := contacts.cond
	nodes := map[uint64]*nodeConfig{}
	for id, node := range cfg.SeedNodes {
		pool := x509.NewCertPool()
		switch node.TransportCert.Type {
		case "ecdsa":
			if !pool.AppendCertsFromPEM([]byte(node.TransportCert.Value)) {
				return nil, fmt.Errorf("network: unable to parse the transport certificate for seed node %d", id)
			}
		default:
			return nil, fmt.Errorf("network: unknown transport.cert type for seed node %d: %q", id, node.TransportCert.Type)
		}
		switch node.SigningKey.Type {
		case "ed25519":
			pubkey, err := b32.DecodeString(node.SigningKey.Value)
			if err != nil {
				return nil, fmt.Errorf("network: unable to decode the signing.key for seed node %d: %s", id, err)
			}
			key, err = signature.LoadPublicKey(signature.Ed25519, pubkey)
			if err != nil {
				return nil, fmt.Errorf("network: unable to load the signing.key for seed node %d: %s", id, err)
			}
		default:
			return nil, fmt.Errorf("network: unknown signing.key type for seed node %d: %q", id, node.SigningKey.Type)
		}
		cond[id] = sync.NewCond(&contacts.mu)
		nodes[id] = &nodeConfig{
			key: key,
			tls: &tls.Config{
				RootCAs:    pool,
				ServerName: fmt.Sprintf("%s/%d", name, id),
			},
		}
	}
	return &Topology{
		contacts:   contacts,
		id:         cfg.ID,
		name:       name,
		nodes:      nodes,
		shardCount: cfg.Shard.Count,
		shardSize:  cfg.Shard.Size,
	}, nil
}
