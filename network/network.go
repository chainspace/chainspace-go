package network // import "chainspace.io/prototype/network"

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base32"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"github.com/grandcat/zeroconf"
	"github.com/lucas-clemente/quic-go"
	"github.com/minio/highwayhash"
)

// Error values.
var (
	ErrNodeWithZeroID = errors.New("network: received invalid Node ID: 0")
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

type contacts struct {
	sync.RWMutex
	data map[uint64]string
}

func (c *contacts) get(nodeID uint64) string {
	c.RLock()
	addr := c.data[nodeID]
	c.RUnlock()
	return addr
}

func (c *contacts) set(nodeID uint64, address string) {
	c.Lock()
	c.data[nodeID] = address
	c.Unlock()
}

type nodeConfig struct {
	key signature.PublicKey
	tls *tls.Config
}

// Topology represents a Chainspace network.
type Topology struct {
	contacts   *contacts
	cxns       map[uint64]quic.Session
	id         string
	mu         sync.RWMutex
	name       string
	nodes      map[uint64]*nodeConfig
	rawID      []byte
	shardCount uint64
	shardSize  uint64
}

// BootstrapFile will use the JSON file at the given path for the initial set of
// addresses for nodes in the network.
func (t *Topology) BootstrapFile(path string) error {
	log.Debug("Bootstrapping network via file", fld.NetworkName(t.name), fld.Path(path))
	return errors.New("network: bootstrapping from a static map is not supported yet")
}

// BootstrapMDNS will try to auto-discover the addresses of initial nodes using
// multicast DNS.
func (t *Topology) BootstrapMDNS() {
	log.Debug("Bootstrapping network via mDNS", fld.NetworkName(t.name))
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			if err := t.bootstrapMDNS(ctx); err != nil {
				log.Error("Unable to start bootstrapping mDNS", log.Err(err))
			}
			select {
			case <-ctx.Done():
				cancel()
			}
		}
	}()
}

func (t *Topology) bootstrapMDNS(ctx context.Context) error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return err
	}
	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for {
			select {
			case entry, ok := <-entries:
				if !ok {
					return
				}
				instance := entry.ServiceRecord.Instance
				if !strings.HasPrefix(instance, "_") {
					continue
				}
				nodeID, err := strconv.ParseUint(instance[1:], 10, 64)
				if err != nil {
					continue
				}
				if len(entry.AddrIPv4) > 0 && entry.Port > 0 {
					addr := fmt.Sprintf("%s:%d", entry.AddrIPv4[0].String(), entry.Port)
					oldAddr := t.contacts.get(nodeID)
					if oldAddr != addr {
						log.Debug("Found node address", fld.NodeID(nodeID), fld.Address(addr))
						t.contacts.set(nodeID, addr)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	service := fmt.Sprintf("_%s._chainspace", strings.ToLower(t.id))
	return resolver.Browse(ctx, service, "local.", entries)
}

// BootstrapStatic will use the given static map of addresses for the initial
// addresses of nodes in the network.
func (t *Topology) BootstrapStatic(addresses map[uint64]string) error {
	log.Debug("Bootstrapping network via a static map", fld.NetworkName(t.name))
	t.contacts.Lock()
	for id, addr := range addresses {
		t.contacts.data[id] = addr
	}
	t.contacts.Unlock()
	return nil
}

// BootstrapURL will use the given URL endpoint to discover the initial
// addresses of nodes in the network.
func (t *Topology) BootstrapURL(endpoint string) error {
	log.Debug("Bootstrapping network via URL", fld.NetworkName(t.name))
	return errors.New("network: bootstrapping from a URL is not supported yet")
}

// Dial opens a connection to a node in the given network. It will block if
// unable to find a routing address for the given node.
func (t *Topology) Dial(ctx context.Context, nodeID uint64) (*Conn, error) {
	t.mu.RLock()
	cfg, cfgExists := t.nodes[nodeID]
	conn, connExists := t.cxns[nodeID]
	t.mu.RUnlock()
	if !cfgExists {
		return nil, fmt.Errorf("network: could not find config for node %d in the %s network", nodeID, t.name)
	}
	if connExists {
		stream, err := conn.OpenStreamSync()
		if err == nil {
			return NewConn(stream), nil
		}
	}
	addr := t.Lookup(nodeID)
	if addr == "" {
		return nil, fmt.Errorf("network: could not find address for node %d", nodeID)
	}
	qcfg := quic.Config{IdleTimeout: 5 * time.Hour, KeepAlive: true}
	conn, err := quic.DialAddrContext(ctx, addr, cfg.tls, &qcfg)
	if err != nil {
		return nil, fmt.Errorf("network: could not connect to node %d: %s", nodeID, err)
	}
	stream, err := conn.OpenStreamSync()
	if err != nil {
		return nil, fmt.Errorf("network: could not open a stream in the connection to %d: %s", nodeID, err)
	}
	t.mu.Lock()
	t.cxns[nodeID] = conn
	t.mu.Unlock()
	return NewConn(stream), nil
}

// Lookup returns the latest host:port address for a given node ID.
func (t *Topology) Lookup(nodeID uint64) string {
	return t.contacts.get(nodeID)
}

// NodesInShard returns a slice of node IDs for the given shard ID.
func (t *Topology) NodesInShard(shardID uint64) []uint64 {
	if shardID == 0 || shardID > t.shardCount {
		log.Fatal("Invalid shard ID", fld.ShardID(shardID), fld.ShardCount(t.shardCount))
	}
	nodes := []uint64{}
	total := t.shardCount * t.shardSize
	for i := shardID; i <= total; i += t.shardCount {
		nodes = append(nodes, i)
	}
	return nodes
}

// SeedPublicKeys returns a map of the signing keys for each of the seed nodes.
func (t *Topology) SeedPublicKeys() map[uint64]signature.PublicKey {
	keys := map[uint64]signature.PublicKey{}
	for nodeID, cfg := range t.nodes {
		keys[nodeID] = cfg.key
	}
	return keys
}

// ShardForKey returns the shard ID for the given object key.
func (t *Topology) ShardForKey(key []byte) uint64 {
	hash := highwayhash.Sum64(key, t.rawID)
	return (hash % t.shardCount) + 1
}

// ShardForNode returns the shard ID for the given node ID.
func (t *Topology) ShardForNode(nodeID uint64) uint64 {
	return ((nodeID - 1) % t.shardCount) + 1
}

// New parses the given network configuration and creates a network topology for
// connecting to nodes.
func New(name string, cfg *config.Network) (*Topology, error) {
	var key signature.PublicKey
	contacts := &contacts{
		data: map[uint64]string{},
	}
	nodes := map[uint64]*nodeConfig{}
	for id, node := range cfg.SeedNodes {
		if id == 0 {
			return nil, ErrNodeWithZeroID
		}
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
		nodes[id] = &nodeConfig{
			key: key,
			tls: &tls.Config{
				RootCAs:    pool,
				ServerName: fmt.Sprintf("%s/%d", name, id),
			},
		}
	}
	rawID, err := b32.DecodeString(cfg.ID)
	if err != nil {
		return nil, err
	}
	expectedID, err := cfg.Hash()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(expectedID, rawID) {
		return nil, fmt.Errorf("network: the given network ID %q does not match the expected value of %q", cfg.ID, b32.EncodeToString(expectedID))
	}
	return &Topology{
		contacts:   contacts,
		cxns:       map[uint64]quic.Session{},
		id:         cfg.ID,
		name:       name,
		nodes:      nodes,
		rawID:      rawID,
		shardCount: uint64(cfg.Shard.Count),
		shardSize:  uint64(cfg.Shard.Size),
	}, nil
}
