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

// Announce represents the configuration for announcing address to peers.
type Announce struct {
	MDNS     bool `yaml:"mdns,omitempty"`
	Registry bool `yaml:"registry,omitempty"`
}

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

// Contracts represents the configuration for the contracts.
type NodeContracts struct {
	Docker bool `yaml:"docker"`
	Manage bool `yaml:"manage"`
}

// HTTP represents the configuration for the REST HTTP API exposed by a node.
type HTTP struct {
	Enabled bool
	Port    int `yaml:"port,omitempty"`
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

// NetConsensus represents the network-wide configuration for the consensus
// protocol.
type NetConsensus struct {
	BlockReferencesSizeLimit   ByteSize      `yaml:"block.references.size.limit"`
	BlockTransactionsSizeLimit ByteSize      `yaml:"block.transactions.size.limit"`
	NonceExpiration            time.Duration `yaml:"nonce.expiration"`
	RoundInterval              time.Duration `yaml:"round.interval"`
	ViewTimeout                int           `yaml:"view.timeout"`
}

// Network represents the configuration of a Chainspace network as a whole.
type Network struct {
	ID         string           `yaml:",omitempty"`
	Consensus  *NetConsensus    `yaml:"consensus"`
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

// Node represents the configuration of an individual node in a Chainspace
// network.
type Node struct {
	Announce          *Announce      `yaml:"announce,omitempty"`
	Bootstrap         *Bootstrap     `yaml:"bootstrap"`
	Broadcast         *Broadcast     `yaml:"broadcast"`
	Connections       *Connections   `yaml:"connections"`
	Consensus         *NodeConsensus `yaml:"consensus"`
	Contracts         *NodeContracts `yaml:"contracts"`
	DisableTransactor bool           `yaml:"disable.transactor,omitempty"`
	HTTP              HTTP           `yaml:"http,omitempty"`
	Logging           *Logging       `yaml:"logging"`
	Pubsub            *Pubsub        `yaml:"pubsub"`
	Registries        []Registry     `yaml:"registries,omitempty"`
	Storage           *Storage       `yaml:"storage"`
}

// NodeConsensus represents the node-specific configuration for the consensus
// protocol.
type NodeConsensus struct {
	DriftTolerance      time.Duration `yaml:"drift.tolerance"`
	InitialWorkDuration time.Duration `yaml:"initial.work.duration"`
	RateLimit           *RateLimit    `yaml:"rate.limit"`
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

// Pubsub represent the configuration for the pub sub system
type Pubsub struct {
	Enabled bool `yaml:"enabled"`
	// if not specified choose random port and announce with mdns
	Port *int `yaml:"port"`
}

// RateLimit represents the configuration for rate-limiting within the system.
type RateLimit struct {
	InitialRate  int     `yaml:"initial.rate"`
	RateDecrease float64 `yaml:"rate.decrease"`
	RateIncrease int     `yaml:"rate.increase"`
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

// DockerContract represent the configuration for a contract running
// inside a docker container
type DockerContract struct {
	Name           string
	Procedures     []string
	Image          string
	Addr           string
	Port           string
	HostPort       string
	HealthCheckURL string
}

type Contract struct {
	Name           string
	Procedures     []string
	Addr           string
	HealthCheckURL string
}

// Contracts represent the global configuration for the contracts
type Contracts struct {
	DockerContracts      []DockerContract `yaml:"contracts.docker"`
	Contracts            []Contract       `yaml:"contracts"`
	DockerMinimalVersion string           `yaml:"docker.minimalversion"`
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

// LoadContracts will read the YAML file at the given path and return the
// corresponding Contracts config.
func LoadContracts(path string) (*Contracts, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Contracts{}
	err = yaml.Unmarshal(data, cfg)

	return cfg, err
}
