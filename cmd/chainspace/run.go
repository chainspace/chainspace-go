package main

import (
	"os"
	"path/filepath"
	"strconv"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/node"
	"go.uber.org/zap"
)

func cmdRun(args []string, usage string) {

	opts := newOpts("run NETWORK_NAME NODE_ID [OPTIONS]", usage)
	configRoot := opts.Flags("-c", "--config-root").Label("PATH").String("path to the chainspace root directory [~/.chainspace]", defaultRootDir())
	runtimeRoot := opts.Flags("-r", "--runtime-root").Label("PATH").String("path to the runtime root directory [~/.chainspace]", defaultRootDir())
	networkName, nodeID := getNetworkNameAndNodeID(opts, args)

	_, err := os.Stat(*configRoot)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatal("Could not find the Chainspace root directory", zap.String("path", *configRoot))
		}
		log.Fatal("Unable to access the Chainspace root directory", zap.String("path", *configRoot), zap.Error(err))
	}

	netPath := filepath.Join(*configRoot, networkName)
	netCfg, err := config.LoadNetwork(filepath.Join(netPath, "network.yaml"))
	if err != nil {
		log.Fatal("Could not load network.yaml", zap.Error(err))
	}

	nodeDir := "node-" + strconv.FormatUint(nodeID, 10)
	nodePath := filepath.Join(netPath, nodeDir)
	nodeCfg, err := config.LoadNode(filepath.Join(nodePath, "node.yaml"))
	if err != nil {
		log.Fatal("Could not load node.yaml", zap.Error(err))
	}

	keys, err := config.LoadKeys(filepath.Join(nodePath, "keys.yaml"))
	if err != nil {
		log.Fatal("Could not load keys.yaml", zap.Error(err))
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
		log.Fatal("Could not start node", zap.Uint64("node.id", nodeID), zap.Error(err))
	}

	wait := make(chan struct{})
	<-wait
}
