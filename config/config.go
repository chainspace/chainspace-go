package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Key struct {
	Algorithm string `yaml:"alg"`
	Value     string `yaml:"value"`
}

type Peer struct {
	Address string `yaml:"address"`
	PubKey  *Key   `yaml:"pubkey"`
}

type Node struct {
	Address string           `yaml:"address"`
	ID      uint64           `yaml:"id"`
	Moniker string           `yaml:"moniker"`
	Peers   map[uint64]*Peer `yaml:"peers"`
	PubKey  *Key             `yaml:"pubkey"`
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

func ParsePrivkey(path string) (*Key, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key := &Key{}
	err = yaml.Unmarshal(data, key)
	return key, err
}
