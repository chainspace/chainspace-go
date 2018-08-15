package main

import (
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"chainspace.io/prototype/node"
	"github.com/tav/golly/process"
)

func cmdRun(args []string, usage string) {
	opts := newOpts("run NETWORK_NAME NODE_ID [OPTIONS]", usage)
	configRoot := opts.Flags("--config-root").Label("PATH").String("path to the chainspace root directory [~/.chainspace]", defaultRootDir())
	consoleLog := opts.Flags("--console-log").Label("LEVEL").String("set the minimum console log level")
	cpuProfile := opts.Flags("--cpu-profile").Label("PATH").String("write a CPU profile to the given file before exiting")
	runtimeRoot := opts.Flags("--runtime-root").Label("PATH").String("path to the runtime root directory [~/.chainspace]", defaultRootDir())
	networkName, nodeID := getNetworkNameAndNodeID(opts, args)

	switch *consoleLog {
	case "error":
		log.ToConsole(log.ErrorLevel)
	case "fatal":
		log.ToConsole(log.FatalLevel)
	case "info":
		log.ToConsole(log.InfoLevel)
	}

	_, err := os.Stat(*configRoot)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatal("Could not find the Chainspace root directory", fld.Path(*configRoot))
		}
		log.Fatal("Unable to access the Chainspace root directory", fld.Path(*configRoot), fld.Err(err))
	}

	netPath := filepath.Join(*configRoot, networkName)
	netCfg, err := config.LoadNetwork(filepath.Join(netPath, "network.yaml"))
	if err != nil {
		log.Fatal("Could not load network.yaml", fld.Err(err))
	}

	nodeDir := "node-" + strconv.FormatUint(nodeID, 10)
	nodePath := filepath.Join(netPath, nodeDir)
	nodeCfg, err := config.LoadNode(filepath.Join(nodePath, "node.yaml"))
	if err != nil {
		log.Fatal("Could not load node.yaml", fld.Err(err))
	}

	keys, err := config.LoadKeys(filepath.Join(nodePath, "keys.yaml"))
	if err != nil {
		log.Fatal("Could not load keys.yaml", fld.Err(err))
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

	var profileFile *os.File
	if *cpuProfile != "" {
		profileFile, err = os.Create(*cpuProfile)
		if err != nil {
			log.Fatal("Could not create CPU profile file", fld.Path(*cpuProfile), fld.Err(err))
		}
		pprof.StartCPUProfile(profileFile)
	}

	s, err := node.Run(cfg)
	if err != nil {
		log.Fatal("Could not start node", fld.NodeID(nodeID), fld.Err(err))
	}

	process.SetExitHandler(func() {
		s.Shutdown()
		if *cpuProfile != "" {
			pprof.StopCPUProfile()
		}
	})

	wait := make(chan struct{})
	<-wait
}
