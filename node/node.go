package node // import "chainspace.io/prototype/node"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	port, err := freeport.UDP(cfg.Node.HostIP)
	if err != nil {
		return nil, err
	}

	// Start listening on the given port.

	// Announce our location to the world.
	for _, meth := range cfg.Node.Announce {
		if meth == "mdns" {
			if err = announceMDNS(cfg.Network.ID, cfg.NodeID, cfg.Node.HostIP, port); err != nil {
				return nil, err
			}
		} else if strings.HasPrefix(meth, "https://") || strings.HasPrefix(meth, "http://") {
			// Announce to the given URL endpoint.
		} else {
			return nil, fmt.Errorf("node: announcement endpoint %q does not start with http:// or https://", meth)
		}
	}

	log.Infof("Node %d of the %s network is now running on port %d", cfg.NodeID, cfg.NetworkName, port)
	log.Infof("Runtime directory: %s", dir)
	return nil, nil

}
