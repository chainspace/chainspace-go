package main

import (
	"os"
	"path/filepath"
	"strconv"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/node"
	"github.com/tav/golly/log"
)

func cmdRun(args []string, usage string) {

	opts := newOpts("run NETWORK_NAME NODE_ID [OPTIONS]", usage)
	configRoot := opts.Flags("-c", "--config-root").Label("PATH").String("path to the chainspace root directory [~/.chainspace]", defaultRootDir())
	runtimeRoot := opts.Flags("-r", "--runtime-root").Label("PATH").String("path to the runtime root directory [~/.chainspace]", defaultRootDir())
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

	nodeDir := "node-" + strconv.FormatUint(nodeID, 10)
	nodePath := filepath.Join(netPath, nodeDir)
	nodeCfg, err := config.LoadNode(filepath.Join(nodePath, "node.yaml"))
	if err != nil {
		log.Fatalf("Could not load node.yaml: %s", err)
	}

	keys, err := config.LoadKeys(filepath.Join(nodePath, "keys.yaml"))
	if err != nil {
		log.Fatalf("Could not load keys.yaml: %s", err)
	}

	root := *configRoot
	if *runtimeRoot != "" {
		root = os.ExpandEnv(*runtimeRoot)
	}

	cfg := &node.Config{
		Directory:   filepath.Join(root, networkName, nodeDir),
		Keys:        keys,
		Network:     netCfg,
		NetworkName: networkName,
		NodeID:      nodeID,
		Node:        nodeCfg,
	}
	if _, err = node.Run(cfg); err != nil {
		log.Fatalf("Could not start node %d: %s", nodeID, err)
	}

	wait := make(chan struct{})
	<-wait

}
