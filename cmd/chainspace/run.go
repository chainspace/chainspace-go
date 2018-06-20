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
	configRoot := opts.Flags("-c", "--config-root").Label("PATH").String("path to the chainspace root directory [~/.chainspace]", defaultRootDir())
	runtimeRoot := opts.Flags("-r", "--runtime-root").Label("PATH").String("path to the runtime root directory [~/.chainspace]")
	networkName, nodeID := getNetworkNameAndNodeID(opts, args)

	_, err := os.Stat(*configRoot)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatal("Could not find the Chainspace root directory at %s", *configRoot)
		}
		log.Fatal("Unable to access the Chainspace root directory at %s: %s", *configRoot, err)
	}

	netPath := filepath.Join(*configRoot, networkName)
	netCfg, err := config.LoadNetwork(filepath.Join(netPath, "network.yaml"))
	if err != nil {
		log.Fatalf("Could not load network.yaml: %s", err)
	}

	nodePath := filepath.Join(netPath, "node-"+strconv.FormatUint(nodeID, 10))
	nodeCfg, err := config.LoadNode(filepath.Join(nodePath, "node.yaml"))
	if err != nil {
		log.Fatalf("Could not load node.yaml: %s", err)
	}

	keys, err := config.LoadKeys(filepath.Join(nodePath, "keys.yaml"))
	if err != nil {
		log.Fatalf("Could not load keys.yaml: %s", err)
	}

	_ = bindAll
	_ = keys
	_ = netCfg
	_ = nodeCfg
	_ = runtimeRoot

}
