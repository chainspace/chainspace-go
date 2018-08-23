package main

import (
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
	"github.com/gogo/protobuf/proto"
	"github.com/tav/golly/process"
)

var (
	txmu   sync.RWMutex
	txrate uint64
)

type loadTracker struct {
	block  *broadcast.Block
	mu     sync.Mutex
	server *node.Server
	total  uint64
}

func (l *loadTracker) handleDeliver(round uint64, blocks []*broadcast.SignedData) {
	l.mu.Lock()
	block := l.block
	for _, signed := range blocks {
		block.Transactions.Count = 0
		block.Transactions.Data = nil
		if err := proto.Unmarshal(signed.Data, block); err != nil {
			log.Fatal("Unable to decode delivered signed block", fld.Err(err))
		}
		l.total += block.Transactions.Count
	}
	l.mu.Unlock()
	l.server.Broadcast.Acknowledge(round)
}

func (l *loadTracker) readCounter() uint64 {
	l.mu.Lock()
	total := l.total
	l.total = 0
	l.mu.Unlock()
	return total
}

func cmdGenLoad(args []string, usage string) {
	opts := newOpts("genload NETWORK_NAME NODE_ID [OPTIONS]", usage)
	configRoot := opts.Flags("--config-root").Label("PATH").String("Path to the chainspace root directory [~/.chainspace]", defaultRootDir())
	cpuProfile := opts.Flags("--cpu-profile").Label("PATH").String("Write a CPU profile to the given file before exiting")
	generators := opts.Flags("--generators").Label("N").Int("Number of generator threads [1]")
	initialRate := opts.Flags("--initial-rate").Label("TXS").Int("Initial number of transactions to send per second [10000]")
	memProfile := opts.Flags("--mem-profile").Label("PATH").String("Write the memory profile to the given file before exiting")
	rateDecrease := opts.Flags("--rate-decr").Label("FACTOR").Float("Multiplicative decrease factor of the queue size [0.8]")
	rateIncrease := opts.Flags("--rate-incr").Label("FACTOR").Int("Additive increase factor of the queue size [1000]")
	runtimeRoot := opts.Flags("--runtime-root").Label("PATH").String("Path to the runtime root directory [~/.chainspace]", defaultRootDir())
	txSize := opts.Flags("--tx-size").Label("SIZE").Int("Size in bytes of the generated transactions [100]")
	networkName, nodeID := getNetworkNameAndNodeID(opts, args)

	if *rateDecrease < 0 || *rateDecrease > 1 {
		log.Fatal(fmt.Sprintf("chainspace genload: the --rate-decr value must be between 0 and 1.0"))
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

	app := &loadTracker{
		block: &broadcast.Block{
			Transactions: &broadcast.Transactions{},
		},
		server: s,
	}

	s.Broadcast.Register(app.handleDeliver)

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

	txrate = uint64(*initialRate / *generators)
	if txrate == 0 {
		txrate = 1
	}

	for i := 0; i < *generators; i++ {
		go genLoad(s, nodeID, *txSize, uint64(*generators))
	}

	manageThroughput(app, netCfg.Shard.Size, uint64(*initialRate), uint64(*rateIncrease), *rateDecrease)

}

func genLoad(s *node.Server, nodeID uint64, txSize int, generators uint64) {
	tick := time.NewTicker(time.Second)
	tx := make([]byte, txSize)
	binary.LittleEndian.PutUint32(tx, uint32(nodeID))
	for {
		<-tick.C
		txmu.RLock()
		rate := txrate
		txmu.RUnlock()
		rate = rate / generators
		if rate == 0 {
			rate = 1
		}
		for i := uint64(0); i < rate; i++ {
			s.Broadcast.AddTransaction(tx, 0)
		}
	}
}

func manageThroughput(l *loadTracker, nodeCount int, startRate uint64, incr uint64, decr float64) {
	var (
		highest  uint64
		maxCount uint64
		maxTotal uint64
	)
	tick := time.NewTicker(time.Second)
	min := startRate / 10
	if min == 0 {
		min = 1
	}
	highestAvg := 0.0
	rate := startRate
	recent := make([]float64, 10)
	idx := 0
	for {
		<-tick.C
		total := l.readCounter()
		recent[idx] = float64(total)
		idx++
		if idx == 10 {
			idx = 0
		}
		avg := 0.0
		for _, val := range recent {
			avg += val
		}
		avg /= 10
		if total < min {
			rate = uint64(float64(rate) * decr)
			if rate < min {
				rate = min
			}
		} else {
			rate += incr
		}
		log.Info("Setting target generation rate for this node", log.Uint64("target.tps", rate))
		txmu.Lock()
		txrate = rate
		txmu.Unlock()
		log.Info("Current transactions/sec", log.Uint64("cur.tps", total))
		if total > highest {
			highest = total
		}
		if avg > highestAvg {
			highestAvg = avg
		}
		log.Info("Highest transactions/sec", log.Uint64("highest.tps", highest))
		maxCount++
		maxTotal += total
		log.Info("Average transactions/sec", log.Uint64("avg.tps", maxTotal/maxCount))
	}
}
