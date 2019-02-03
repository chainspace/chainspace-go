package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"sync"

	"chainspace.io/chainspace-go/blockmania"
	"chainspace.io/chainspace-go/broadcast"
	"chainspace.io/chainspace-go/config"
	"chainspace.io/chainspace-go/internal/log"
	"chainspace.io/chainspace-go/internal/log/fld"
	"chainspace.io/chainspace-go/network"
	"github.com/tav/golly/process"
)

func cmdInterpret(args []string, usage string) {
	opts := newOpts("interpret NETWORK_NAME NODE_ID [OPTIONS]", usage)
	configRoot := opts.Flags("--config-root").Label("PATH").String("Path to the chainspace root directory [~/.chainspace]", defaultRootDir())
	consoleLog := opts.Flags("--console-log").Label("LEVEL").String("Set the minimum console log level")
	cpuProfile := opts.Flags("--cpu-profile").Label("PATH").String("Write a CPU profile to the given file before exiting")
	memProfile := opts.Flags("--mem-profile").Label("PATH").String("Write the memory profile to the given file before exiting")
	runtimeRoot := opts.Flags("--runtime-root").Label("PATH").String("Path to the runtime root directory [~/.chainspace]", defaultRootDir())
	networkName, nodeID := getNetworkNameAndNodeID(opts, args)

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

	top, err := network.New(networkName, netCfg)
	if err != nil {
		log.Fatal("Unable to initialise the network topology", fld.Err(err))
	}

	shardID := top.ShardForNode(nodeID)
	nodes := top.NodesInShard(shardID)
	cfg := &blockmania.Config{
		Nodes:  nodes,
		SelfID: nodeID,
	}

	dir := filepath.Join(*runtimeRoot, networkName, "node-"+strconv.FormatUint(nodeID, 10))
	out, err := os.Create(filepath.Join(dir, "interpreted.state"))
	if err != nil {
		log.Fatal("Could not create the interpreted.state file", fld.Err(err))
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
	}

	closed := false
	mu := sync.Mutex{}

	cb := func(res *blockmania.Interpreted) {
		blocks := make([]string, len(res.Blocks))
		sort.Slice(res.Blocks, func(i, j int) bool {
			return res.Blocks[i].Node < res.Blocks[j].Node
		})
		for i, block := range res.Blocks {
			blocks[i] = block.String()
		}
		log.Info("Got interpreted results", fld.Round(res.Round), log.Uint64("consumed", res.Consumed), log.Strings("blocks", blocks))
		mu.Lock()
		if !closed {
			data, err := json.Marshal(res)
			if err != nil {
				log.Fatal("Could not encode interpreted results as JSON", fld.Err(err))
			}
			data = append(data, 0x00, '\n')
			n, err := out.Write(data)
			if err != nil {
				log.Fatal("Could not write encoded interpreted results to file", fld.Err(err))
			}
			if n != len(data) {
				log.Fatal("Write to file of encoded interpreted results was not comprehensive")
			}
		}
		mu.Unlock() // TODO: Jeremy, there's a lot going on between mutex locks. Should we add `defer` blocks to all the mutexes to ensure they unlock?
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
		if *cpuProfile != "" {
			pprof.StopCPUProfile()
		}
		mu.Lock()
		out.Close()
		closed = true
		mu.Unlock()
	})

	graph := blockmania.New(context.Background(), cfg, cb)
	err = broadcast.Replay(dir, nodeID, graph, 0, 10)
	if err != nil {
		log.Fatal("Could not replay graph", fld.Err(err))
	}

	wait := make(chan struct{})
	<-wait
}
