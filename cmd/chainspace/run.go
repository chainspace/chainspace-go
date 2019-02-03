package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"

	"chainspace.io/chainspace-go/checker"
	"chainspace.io/chainspace-go/config"
	"chainspace.io/chainspace-go/contracts"
	"chainspace.io/chainspace-go/internal/crypto/signature"
	"chainspace.io/chainspace-go/internal/freeport"
	"chainspace.io/chainspace-go/internal/log"
	"chainspace.io/chainspace-go/internal/log/fld"
	"chainspace.io/chainspace-go/node"
	"chainspace.io/chainspace-go/rest"

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

	var s *node.Server
	var rstsrv *rest.Service
	if !*checkerOnly {
		s, err = node.Run(cfg)
		if err != nil {
			log.Fatal("Could not start node", fld.NodeID(nodeID), fld.Err(err))
		}
	} else {
		rstsrv = runCheckerOnly(cfg)
	}
	_ = rstsrv

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
		if s != nil {
			s.Shutdown()
		}
		if *cpuProfile != "" {
			pprof.StopCPUProfile()
		}
	})

	wait := make(chan struct{})
	<-wait
}

func runCheckerOnly(cfg *node.Config) *rest.Service {
	maxPayload, err := cfg.Network.MaxPayload.Int()
	if err != nil {
		log.Fatal("invalid maxpayload", fld.Err(err))
	}
	var key signature.KeyPair
	switch cfg.Keys.SigningKey.Type {
	case "ed25519":
		pubkey, err := b32.DecodeString(cfg.Keys.SigningKey.Public)
		if err != nil {
			log.Fatal("node: could not decode the base32-encoded public signing.key from config", fld.Err(err))
		}
		privkey, err := b32.DecodeString(cfg.Keys.SigningKey.Private)
		if err != nil {
			log.Fatal("node: could not decode the base32-encoded private signing.key from config", fld.Err(err))
		}
		key, err = signature.LoadKeyPair(signature.Ed25519, append(pubkey, privkey...))
		if err != nil {
			log.Fatal("node: unable to load the signing.key from config", fld.Err(err))
		}
	default:
		log.Fatal("node: unknown type of signing.key found in config", log.String("keytype", cfg.Keys.SigningKey.Type))
	}

	// start the different contracts
	cts, err := contracts.New(cfg.Contracts)
	if err != nil {
		log.Fatal("unable to instantiate contacts", fld.Err(err))
	}

	// initialize the contracts
	if cfg.Node.Contracts.Manage {
		err = cts.Start()
		if err != nil {
			log.Fatal("unable to start contracts", fld.Err(err))
		}
	}

	// ensure all the contracts are working
	if err := cts.EnsureUp(); err != nil {
		log.Fatal("some contracts are unavailable, run `chainspace contracts <yournetworkname> create`", fld.Err(err))
	}

	tcheckers := []checker.Checker{}
	checkers := cts.GetCheckers()
	for _, v := range checkers {
		tcheckers = append(tcheckers, v)
	}

	checkercfg := &checker.Config{
		Checkers:   tcheckers,
		SigningKey: cfg.Keys.SigningKey,
	}
	checkr, err := checker.New(checkercfg)
	if err != nil {
		log.Fatal("node unable to instantiate checker service: %v", fld.Err(err))
	}
	var rport int
	if cfg.Node.HTTP.Port > 0 {
		rport = cfg.Node.HTTP.Port
	} else {
		rport, _ = freeport.TCP("")
	}
	restsrvcfg := &rest.Config{
		Addr:        "",
		Checker:     checkr,
		CheckerOnly: cfg.CheckerOnly,
		Key:         key,
		MaxPayload:  config.ByteSize(maxPayload),
		Port:        rport,
		SBACOnly:    cfg.SBACOnly,
		SelfID:      cfg.NodeID,
	}
	rstsrv := rest.New(restsrvcfg)
	return rstsrv
}
