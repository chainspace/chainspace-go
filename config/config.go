package config // import "chainspace.io/prototype/config"

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Key struct {
	Type    string
	Public  string
	Private string `yaml:",omitempty"`
}

type Keys struct {
	SigningKey    *Key `yaml:"signing.key"`
	TransportCert *Key `yaml:"transport.cert"`
}

type Network struct {
	Shards    int
	SeedNodes map[uint64]*Peer `yaml:"seed.nodes"`
}

type Node struct {
	Address   string
	Bootstrap string
}

type Peer struct {
	SigningKey    *PeerKey `yaml:"signing.key"`
	TransportCert *PeerKey `yaml:"transport.cert"`
}

type PeerKey struct {
	Type  string
	Value string
}

func ParseKeys(path string) (*Keys, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Keys{}
	err = yaml.Unmarshal(data, cfg)
	return cfg, err
}

func ParseNetwork(path string) (*Network, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	network := &Network{}
	err = yaml.Unmarshal(data, network)
	return network, err
}

func ParseNode(path string) (*Node, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Node{}
	err = yaml.Unmarshal(data, cfg)
	return cfg, err
}
