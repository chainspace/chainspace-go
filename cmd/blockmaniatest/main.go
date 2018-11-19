package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/node"
)

var (
	configRoot  string
	runtimeRoot string
	networkName string
	nodeID      int
	txSize      int
	tps         int
)

func init() {
	flag.StringVar(&configRoot, "config-root", defaultRootDir(), "Path to the chainspace root directory")
	flag.StringVar(&runtimeRoot, "runtime-root", defaultRootDir(), "Path to the runtime root directory")
	flag.StringVar(&networkName, "network", "", "Name of the network to use")
	flag.IntVar(&nodeID, "nodeid", 0, "Node id to start")
	flag.IntVar(&txSize, "tx-size", 100, "Size in bytes of the generated transaction")
	flag.IntVar(&tps, "tps", 10000, "Maximum number of transaction generated per seconds")
	flag.Parse()
}

func defaultRootDir() string {
	return os.ExpandEnv("$HOME/.chainspace")
}

func main() {
	var haserr bool
	if len(networkName) <= 0 {
		fmt.Printf("missing mandatory argument `-network`\n")
		haserr = true
	}
	if nodeID <= 0 {
		fmt.Printf("missing mandatory argument `-nodeid`\n")
		haserr = true
	}
	if haserr == true {
		flag.Usage()
		os.Exit(1)
	}

	_, err := os.Stat(configRoot)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Could not find the Chainspace root directory: %v\n", configRoot)
			os.Exit(1)
		}
		fmt.Printf("Unable to access the Chainspace root directory at %v: %v\n", configRoot, err)
		os.Exit(1)
	}

	netPath := filepath.Join(configRoot, networkName)
	netCfg, err := config.LoadNetwork(filepath.Join(netPath, "network.yaml"))
	if err != nil {
		fmt.Printf("Could not load network.yaml: %v\n", err)
		os.Exit(1)
	}

	nodeDir := "node-" + strconv.FormatUint(uint64(nodeID), 10)
	nodePath := filepath.Join(netPath, nodeDir)
	nodeCfg, err := config.LoadNode(filepath.Join(nodePath, "node.yaml"))
	if err != nil {
		fmt.Printf("Could not load node.yaml: %v\n", err)
		os.Exit(1)
	}

	keys, err := config.LoadKeys(filepath.Join(nodePath, "keys.yaml"))
	if err != nil {
		fmt.Printf("Could not load keys.yaml: %v\n", err)
		os.Exit(1)
	}

	root := configRoot
	if runtimeRoot != "" {
		root = os.ExpandEnv(runtimeRoot)
	}

	cfg := &node.Config{
		Directory:   filepath.Join(root, networkName, nodeDir),
		Keys:        keys,
		Network:     netCfg,
		NetworkName: networkName,
		NodeID:      uint64(nodeID),
		Node:        nodeCfg,
	}
	_ = cfg

	nodeCfg.DisableSBAC = true
	nodeCfg.Logging.FileLevel = log.FatalLevel
	nodeCfg.Logging.ConsoleLevel = log.FatalLevel
}
