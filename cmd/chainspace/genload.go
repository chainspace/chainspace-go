package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"

	"chainspace.io/prototype/broadcast"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"chainspace.io/prototype/node"
	"github.com/tav/golly/process"
)

var txpool = &sync.Pool{
	New: func() interface{} {
		return &broadcast.TransactionData{}
	},
}

type loadTracker struct {
	counter int
	server  *node.Server
}

func (l *loadTracker) DeliverEnd(round uint64) {
	log.Info(fmt.Sprintf("%d transactions delivered", l.counter))
	l.server.Broadcast.Acknowledge(round)
}

func (l *loadTracker) DeliverStart(round uint64) {
	l.counter = 0
}

func (l *loadTracker) DeliverTransaction(tx *broadcast.TransactionData) {
	txpool.Put(tx)
	l.counter++
}

func cmdGenLoad(args []string, usage string) {
	opts := newOpts("genload NETWORK_NAME NODE_ID [OPTIONS]", usage)
	configRoot := opts.Flags("--config-root").Label("PATH").String("path to the chainspace root directory [~/.chainspace]", defaultRootDir())
	cpuProfile := opts.Flags("--cpu-profile").Label("PATH").String("write a CPU profile to the given file before exiting")
	memProfile := opts.Flags("--mem-profile").Label("PATH").String("write the memory profile to the given file before exiting")
	runtimeRoot := opts.Flags("--runtime-root").Label("PATH").String("path to the runtime root directory [~/.chainspace]", defaultRootDir())
	txInterval := opts.Flags("--tx-interval").Label("DURATION").Duration("interval between sending every transaction [10us]")
	txSize := opts.Flags("--tx-size").Label("SIZE").Int("size in bytes of the generated transactions [100]")
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

	if *cpuProfile != "" {
		profileFile, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal("Could not create CPU profile file", fld.Path(*cpuProfile), fld.Err(err))
		}
		pprof.StartCPUProfile(profileFile)
	}

	nodeCfg.DisableTransactor = true
	nodeCfg.Logging.FileLevel = 0
	log.ToConsole(log.InfoLevel)

	if *txSize < 16 {
		log.Fatal("The --tx-size value must be at least 16 bytes")
	}

	s, err := node.Run(cfg)
	if err != nil {
		log.Fatal("Could not start node", fld.NodeID(nodeID), fld.Err(err))
	}

	s.Broadcast.Register(&loadTracker{
		server: s,
	})

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

	genLoad(s, *txSize, *txInterval)

}

func genLoad(s *node.Server, size int, interval time.Duration) {
	counter := uint64(0)
	ident := make([]byte, 8)
	_, err := rand.Read(ident)
	if err != nil {
		log.Fatal("Unable to generate random ident for load generator", fld.Err(err))
	}
	for {
		counter++
		tx := txpool.Get().(*broadcast.TransactionData)
		if len(tx.Data) != size {
			tx.Data = make([]byte, size)
		}
		copy(tx.Data[:8], ident)
		binary.LittleEndian.PutUint64(tx.Data[8:16], counter)
		s.Broadcast.AddTransaction(tx)
		time.Sleep(interval)
	}
}

func trackLoad(s *node.Server) {}
