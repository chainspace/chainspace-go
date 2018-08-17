package main

import (
	"bytes"
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

var (
	txmu   sync.RWMutex
	txrate int
)

type loadTracker struct {
	mu     sync.Mutex
	self   int
	nodeID []byte
	server *node.Server
	total  int
}

func (l *loadTracker) DeliverEnd(round uint64) {
	l.mu.Unlock()
	l.server.Broadcast.Acknowledge(round)
}

func (l *loadTracker) DeliverStart(round uint64) {
	l.mu.Lock()
}

func (l *loadTracker) DeliverTransaction(tx *broadcast.TransactionData) {
	txpool.Put(tx)
	l.total++
	if bytes.Equal(tx.Data[:4], l.nodeID) {
		l.self++
	}
}

func (l *loadTracker) readCounter() (int, int) {
	l.mu.Lock()
	self := l.self
	total := l.total
	l.self = 0
	l.total = 0
	l.mu.Unlock()
	return self, total
}

func cmdGenLoad(args []string, usage string) {
	opts := newOpts("genload NETWORK_NAME NODE_ID [OPTIONS]", usage)
	configRoot := opts.Flags("--config-root").Label("PATH").String("path to the chainspace root directory [~/.chainspace]", defaultRootDir())
	cpuProfile := opts.Flags("--cpu-profile").Label("PATH").String("write a CPU profile to the given file before exiting")
	initialRate := opts.Flags("--initial-rate").Label("TXS").Int("initial number of transactions to send per second [10000]")
	memProfile := opts.Flags("--mem-profile").Label("PATH").String("write the memory profile to the given file before exiting")
	rateDecrease := opts.Flags("--rate-decr").Label("FACTOR").String("multiplicative decrease factor of the queue size [0.8]")
	rateIncrease := opts.Flags("--rate-incr").Label("FACTOR").Int("additive increase factor of the queue size [1000]")
	runtimeRoot := opts.Flags("--runtime-root").Label("PATH").String("path to the runtime root directory [~/.chainspace]", defaultRootDir())
	txSize := opts.Flags("--tx-size").Label("SIZE").Int("size in bytes of the generated transactions [100]")
	networkName, nodeID := getNetworkNameAndNodeID(opts, args)

	rateDecr, err := strconv.ParseFloat(*rateDecrease, 64)
	if err != nil {
		log.Fatal(fmt.Sprintf("chainspace genload: couldn't convert --rate-decr value '%s' to a float", *rateDecrease))
	}
	if rateDecr < 0 || rateDecr > 1 {
		log.Fatal(fmt.Sprintf("chainspace genload: the --rate-decr value must be between 0 and 1.0"))
	}

	_, err = os.Stat(*configRoot)
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

	nenc := make([]byte, 4)
	binary.LittleEndian.PutUint32(nenc, uint32(nodeID))
	app := &loadTracker{
		nodeID: nenc,
		server: s,
	}

	s.Broadcast.Register(app)

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

	txrate = *initialRate
	go manageThroughput(app, netCfg.Shard.Size, *initialRate, *rateIncrease, rateDecr)
	genLoad(s, nodeID, *txSize)

}

func genLoad(s *node.Server, nodeID uint64, txSize int) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(nodeID))
	tick := time.NewTicker(time.Second)
	for {
		<-tick.C
		txmu.RLock()
		rate := txrate
		txmu.RUnlock()
		for i := 0; i < rate; i++ {
			tx := txpool.Get().(*broadcast.TransactionData)
			if len(tx.Data) != txSize {
				tx.Data = make([]byte, txSize)
			}
			copy(tx.Data, buf)
			s.Broadcast.AddTransaction(tx)
		}
	}
}

func manageThroughput(l *loadTracker, nodeCount int, startRate int, incr int, decr float64) {
	tick := time.NewTicker(time.Second)
	min := startRate / 10
	if min == 0 {
		min = 1
	}
	rate := startRate
	recent := make([]float64, 10)
	txs := make([]int, 10)
	for i := range recent {
		recent[i] = float64(startRate)
		txs[i] = 0
	}
	idx := 0
	prev := float64(startRate)
	for {
		<-tick.C
		self, total := l.readCounter()
		recent[idx] = float64(self)
		txs[idx] = total
		idx++
		if idx >= 10 {
			idx -= 10
		}
		avg := 0.0
		for _, val := range recent {
			avg += val
		}
		avg /= 10
		if self == 0 || avg < prev {
			rate = int(float64(rate) * decr)
			if rate < min {
				rate = min
			}
		} else {
			rate += incr
		}
		prev = avg
		log.Info("Setting target generation rate for this node", log.Int("target.tps", rate))
		txmu.Lock()
		txrate = rate
		txmu.Unlock()
		avgTxs := 0
		for _, val := range txs {
			avgTxs += val
		}
		avgTxs /= 10
		log.Info("Transactions received in the last second", log.Int("cur.tps", total))
		log.Info("Average transactions/sec in the last 10 seconds", log.Int("avg.tps", avgTxs))
	}
}
