package main

import (
	"os"
	"path/filepath"
	"strconv"

	"chainspace.io/prototype/config"
	"github.com/tav/golly/log"
)

func cmdRun(args []string, usage string) {

	opts := newOpts("run NETWORK_NAME NODE_ID [OPTIONS]", usage)
	bindAll := opts.Flags("-b", "--bind-all").Bool("override host.ip in node.yaml and bind to all interfaces instead")
	rootDir := opts.Flags("-r", "--root").Label("DIR").String("path to the chainspace root directory", defaultRootDir())
	networkName, nodeID := getNetworkNameAndNodeID(opts, args)

	_, err := os.Stat(*rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatal("Could not find the Chainspace root directory at %s", *rootDir)
		}
		log.Fatal("Unable to access the Chainspace root directory at %s: %s", *rootDir, err)
	}

	netPath := filepath.Join(*rootDir, networkName)
	netCfg, err := config.ParseNetwork(filepath.Join(netPath, "network.yaml"))
	if err != nil {
		log.Fatalf("Could not parse network.yaml: %s", err)
	}

	nodePath := filepath.Join(netPath, "node-"+strconv.FormatUint(nodeID, 10))
	nodeCfg, err := config.ParseNode(filepath.Join(nodePath, "node.yaml"))
	if err != nil {
		log.Fatalf("Could not parse node.yaml: %s", err)
	}

	keys, err := config.ParseKeys(filepath.Join(nodePath, "keys.yaml"))
	if err != nil {
		log.Fatalf("Could not parse keys.yaml: %s", err)
	}

	_ = bindAll
	_ = keys
	_ = netCfg
	_ = nodeCfg

}
