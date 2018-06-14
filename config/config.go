package config // import "chainspace.io/prototype/config"

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Keys struct {
	SigningKey    *SigningKey    `yaml:"signing.key"`
	TransportCert *TransportCert `yaml:"transport.cert"`
}

type Node struct {
	Address string
	ID      uint64 `yaml:"id"`
}

type Peer struct {
	Address       string
	SigningKey    *PeerKey `yaml:"signing.key"`
	TransportCert *PeerKey `yaml:"transport.cert"`
}

type PeerKey struct {
	Type  string
	Value string
}

type SigningKey struct {
	Type    string
	Public  string
	Private string `yaml:",omitempty"`
}

type TransportCert struct {
	Type    string
	Public  string
	Private string
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

func ParseNode(path string) (*Node, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Node{}
	err = yaml.Unmarshal(data, cfg)
	return cfg, err
}

func ParsePeers(path string) (map[uint64]*Peer, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	peers := map[uint64]*Peer{}
	err = yaml.Unmarshal(data, peers)
	return peers, err
}
