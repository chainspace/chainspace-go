package node // import "chainspace.io/prototype/node"

import (
	"errors"
	"os"
	"path/filepath"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/freeport"
	"chainspace.io/prototype/network"
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
	top *network.Topology
}

// Run initialises a node with the given config.
func Run(cfg *Config) (*Server, error) {
	var err error
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
	port, err := freeport.UDP(cfg.Node.HostIP)
	if err != nil {
		return nil, err
	}
	log.Infof("Node %d of the %s network is now running on port %d", cfg.NodeID, cfg.NetworkName, port)
	log.Infof("Runtime directory: %s", dir)
	return nil, nil
}
