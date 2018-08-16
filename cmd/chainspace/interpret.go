package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"chainspace.io/prototype/broadcast"
	"chainspace.io/prototype/byzco"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"chainspace.io/prototype/network"
	"github.com/tav/golly/process"
)

func cmdInterpret(args []string, usage string) {
	opts := newOpts("interpret NETWORK_NAME NODE_ID [OPTIONS]", usage)
	configRoot := opts.Flags("--config-root").Label("PATH").String("path to the chainspace root directory [~/.chainspace]", defaultRootDir())
	consoleLog := opts.Flags("--console-log").Label("LEVEL").String("set the minimum console log level")
	runtimeRoot := opts.Flags("--runtime-root").Label("PATH").String("path to the runtime root directory [~/.chainspace]", defaultRootDir())
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
	cfg := &byzco.Config{
		Nodes:  nodes,
		SelfID: nodeID,
	}

	dir := filepath.Join(*runtimeRoot, networkName, "node-"+strconv.FormatUint(nodeID, 10))
	out, err := os.Create(filepath.Join(dir, "interpreted.state"))
	if err != nil {
		log.Fatal("Could not create the interpreted.state file", fld.Err(err))
	}

	closed := false
	mu := sync.Mutex{}
	process.SetExitHandler(func() {
		mu.Lock()
		out.Close()
		closed = true
		mu.Unlock()
	})

	cb := func(res *byzco.Interpreted) {
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
		mu.Unlock()
	}

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

	graph := byzco.New(context.Background(), cfg, cb)
	err = broadcast.Replay(dir, nodeID, graph, 0, 10)
	if err != nil {
		log.Fatal("Could not replay graph", fld.Err(err))
	}

	wait := make(chan struct{})
	<-wait
}
