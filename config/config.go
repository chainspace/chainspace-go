package config // import "chainspace.io/prototype/config"

// NOTE(tav): The order of some of the struct fields are purposefully not in
// alphabetical order so as to generate a more pleasing ordering when serialised
// to YAML.
import (
	"crypto/sha512"
	"io/ioutil"
	"strings"
	"time"

	"chainspace.io/prototype/log"
	"gopkg.in/yaml.v2"
)

// Bootstrap represents the configuration for bootstrapping peer addresses.
type Bootstrap struct {
	File     string            `yaml:",omitempty"`
	MDNS     bool              `yaml:",omitempty"`
	Registry bool              `yaml:",omitempty"`
	Static   map[uint64]string `yaml:",omitempty"`
}

// Broadcast represents the configuration for maintaining the shard broadcast by
// a node.
type Broadcast struct {
	InitialBackoff time.Duration `yaml:"initial.backoff"`
	MaxBackoff     time.Duration `yaml:"max.backoff"`
}

// Connections represents the configuration for network connections. MaxPayload
// when used as part of the Network.NodeConnections configuration, also
// implicitly defines the maximum size of blocks.
type Connections struct {
	ReadTimeout  time.Duration `yaml:"read.timeout"`
	WriteTimeout time.Duration `yaml:"write.timeout"`
}

// Consensus represents the configuration for the consensus protocol.
type Consensus struct {
	BlockReferencesSizeLimit   ByteSize      `yaml:"block.references.size.limit"`
	BlockTransactionsSizeLimit ByteSize      `yaml:"block.transactions.size.limit"`
	NonceExpiration            time.Duration `yaml:"nonce.expiration"`
	RoundInterval              time.Duration `yaml:"round.interval"`
	ViewTimeout                int           `yaml:"view.timeout"`
}

// Key represents a cryptographic key of some kind.
type Key struct {
	Type    string
	Public  string
	Private string `yaml:",omitempty"`
}

// Keys represents the configuration that individual nodes use for their various
// cryptographic keys.
type Keys struct {
	SigningKey    *Key `yaml:"signing.key"`
	TransportCert *Key `yaml:"transport.cert"`
}

// Logging represents the logging configuration for individual nodes.
type Logging struct {
	ConsoleLevel log.Level `yaml:"console.level,omitempty"`
	FileLevel    log.Level `yaml:"file.level,omitempty"`
	FilePath     string    `yaml:"file.path,omitempty"`
	ServerHost   string    `yaml:"server.host,omitempty"`
	ServerLevel  log.Level `yaml:"server.level,omitempty"`
	ServerToken  string    `yaml:"server.token,omitempty"`
}

// Network represents the configuration of a Chainspace network as a whole.
type Network struct {
	ID         string           `yaml:",omitempty"`
	Consensus  *Consensus       `yaml:"consensus"`
	MaxPayload ByteSize         `yaml:"max.payload"`
	SeedNodes  map[uint64]*Peer `yaml:"seed.nodes"`
	Shard      *Shard
}

// Hash returns the SHA-512/256 hash of the network's seed configuration by
// first encoding it into YAML. The returned hash should then be used as the ID
// for the network.
func (n *Network) Hash() ([]byte, error) {
	data, err := yaml.Marshal(&Network{
		SeedNodes: n.SeedNodes,
		Shard:     n.Shard,
	})
	if err != nil {
		return nil, err
	}
	h := sha512.New512_256()
	if _, err = h.Write(data); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// Registry represents the configuration of an individual network registry
// server.
type Registry struct {
	Host  string
	Token string
}

// URL returns the registry's root URL including the scheme.
func (r Registry) URL() string {
	if strings.HasPrefix(r.Host, "localhost:") || r.Host == "localhost" {
		return "http://" + r.Host + "/"
	}
	return "https://" + r.Host + "/"
}

// Announce represents the configuration for announcing address to peers.
type Announce struct {
	MDNS     bool `yaml:"mdns,omitempty"`
	Registry bool `yaml:"registry,omitempty"`
}

// Node represents the configuration of an individual node in a Chainspace
// network.
type Node struct {
	Announce          *Announce `yaml:"announce,omitempty"`
	Bootstrap         *Bootstrap
	Broadcast         *Broadcast
	Connections       *Connections `yaml:"connections"`
	DisableTransactor bool         `yaml:"disable.transactor,omitempty"`
	HTTP              HTTP
	Logging           *Logging
	Storage           *Storage
	Registries        []Registry `yaml:registry,omitempty`
}

// HTTP represents the configuration for the rest http api exposed by a node
type HTTP struct {
	Enabled bool
	Port    *int `yaml:"port,omitempty"`
}

// Peer represents the cryptographic public keys of a node in a Chainspace
// network.
type Peer struct {
	SigningKey    *PeerKey `yaml:"signing.key"`
	TransportCert *PeerKey `yaml:"transport.cert"`
}

// PeerKey represents a cryptographic public key of some kind.
type PeerKey struct {
	Type  string
	Value string
}

// Shard represents the configuration of the sharding system within a given
// Chainspace network.
type Shard struct {
	Count int
	Size  int
}

// Storage represents the configuration for a node's underlying storage
// mechanism.
type Storage struct {
	Type string
}

// LoadKeys will read the YAML file at the given path and return the
// corresponding Keys config.
func LoadKeys(path string) (*Keys, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Keys{}
	err = yaml.Unmarshal(data, cfg)
	return cfg, err
}

// LoadNetwork will read the YAML file at the given path and return the
// corresponding Network config.
func LoadNetwork(path string) (*Network, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	network := &Network{}
	err = yaml.Unmarshal(data, network)
	return network, err
}

// LoadNode will read the YAML file at the given path and return the
// corresponding Node config.
func LoadNode(path string) (*Node, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Node{}
	err = yaml.Unmarshal(data, cfg)
	if cfg.Storage == nil || len(cfg.Storage.Type) <= 0 {
		cfg.Storage = &Storage{"memstore"}
	}

	return cfg, err
}
