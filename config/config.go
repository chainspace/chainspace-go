package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type KeyPair struct {
	Algorithm string `yaml:"alg"`
	PrivKey   string `yaml:"privkey"`
	PubKey    string `yaml:"pubkey"`
}

type Node struct {
	Address string `yaml:"address"`
	ID      uint64 `yaml:"id"`
	Moniker string `yaml:"moniker"`
}

type Peer struct {
	Address string     `yaml:"address"`
	PubKey  *PublicKey `yaml:"pubkey"`
}

type PublicKey struct {
	Algorithm string `yaml:"alg"`
	Value     string `yaml:"value"`
}

func ParseKeyPair(path string) (*KeyPair, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &KeyPair{}
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
