package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/node"

	"github.com/tav/golly/process"
)

func cmdRun(args []string, usage string) {
	opts := newOpts("run NETWORK_NAME NODE_ID [OPTIONS]", usage)
	configRoot := opts.Flags("--config-root").Label("PATH").String("Path to the chainspace root directory [~/.chainspace]", defaultRootDir())
	consoleLog := opts.Flags("--console-log").Label("LEVEL").String("Set the minimum console log level")
	cpuProfile := opts.Flags("--cpu-profile").Label("PATH").String("Write a CPU profile to the given file before exiting")
	fileLog := opts.Flags("--file-log").Label("LEVEL").String("Set the minimum file log level")
	memProfile := opts.Flags("--mem-profile").Label("PATH").String("Write the memory profile to the given file before exiting")
	runtimeRoot := opts.Flags("--runtime-root").Label("PATH").String("Path to the runtime root directory [~/.chainspace]", defaultRootDir())
	sbacOnly := opts.Flags("--sbac-only").Label("BOOL").Bool("Start the node as a node only")
	checkerOnly := opts.Flags("--checker-only").Label("BOOL").Bool("Start the node as a checker node only")

	networkName, nodeID := getNetworkNameAndNodeID(opts, args)

	if *sbacOnly == true && *checkerOnly == true {
		log.Fatal("Cannot set the node as node-only and checker-only at the same time")
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

	cts, err := config.LoadContracts(filepath.Join(netPath, "contracts.yaml"))
	if err != nil {
		log.Fatal("Could not load contracts.yaml", fld.Err(err))
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
		Contracts:   cts,
		SBACOnly:    *sbacOnly,
		CheckerOnly: *checkerOnly,
	}

	if *cpuProfile != "" {
		profileFile, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal("Could not create CPU profile file", fld.Path(*cpuProfile), fld.Err(err))
		}
		pprof.StartCPUProfile(profileFile)
	}

	if *consoleLog != "" {
		switch *consoleLog {
		case "debug":
			log.ToConsole(log.DebugLevel)
		case "error":
			log.ToConsole(log.ErrorLevel)
		case "fatal":
			log.ToConsole(log.FatalLevel)
		case "info":
			log.ToConsole(log.InfoLevel)
		default:
			log.Fatal("Unknown --console-log level: " + *consoleLog)
		}
	} else {
		log.ToConsole(nodeCfg.Logging.ConsoleLevel)
	}

	if *fileLog != "" {
		switch *fileLog {
		case "debug":
			nodeCfg.Logging.FileLevel = log.DebugLevel
		case "error":
			nodeCfg.Logging.FileLevel = log.ErrorLevel
		case "fatal":
			nodeCfg.Logging.FileLevel = log.FatalLevel
		case "info":
			nodeCfg.Logging.FileLevel = log.InfoLevel
		default:
			log.Fatal("Unknown --file-log level: " + *fileLog)
		}
	}

	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()

	s, err := node.Run(cfg)
	if err != nil {
		log.Fatal("Could not start node", fld.NodeID(nodeID), fld.Err(err))
	}

	process.SetExitHandler(func() {
		if *memProfile != "" {
			f, err := os.Create(*memProfile)
			if err != nil {
				log.Fatal("Could not create memory profile file", fld.Path(*memProfile), fld.Err(err))
			}
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Fatal("Could not write memory profile", fld.Err(err))
			}
			f.Close()
		}
		s.Shutdown()
		if *cpuProfile != "" {
			pprof.StopCPUProfile()
		}
	})

	wait := make(chan struct{})
	<-wait
}
