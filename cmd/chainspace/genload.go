package main

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"chainspace.io/prototype/broadcast"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"chainspace.io/prototype/node"
	"github.com/gogo/protobuf/proto"
	"github.com/tav/golly/process"
)

type txTimeTracker struct {
	mu     sync.Mutex
	lasttx int64
	tot    int64
	txc    int64

	curGen uint64
}

func (t *txTimeTracker) handleTxTime(round uint64, blocks []*broadcast.SignedData) {
	var u uint32
	usize := unsafe.Sizeof(u)
	now := time.Now()
	minDuration := time.Hour

	t.mu.Lock()
	for _, signed := range blocks {
		block, err := signed.Block()
		if err != nil {
			log.Fatal("Unable to decode delivered block", fld.Err(err))
		}
		it := block.Iter()
		for it.Valid() {
			t.txc += 1
			it.Next()
			wsize := binary.LittleEndian.Uint32(it.TxData[usize : usize+usize])
			ti, err := time.Parse(time.RFC3339Nano, string(it.TxData[usize+usize:uint32(usize)+uint32(usize)+wsize]))
			if err != nil {
				log.Fatal("Unable to parse date", fld.Err(err))
			}
			since := now.Sub(ti)
			if minDuration > since {
				minDuration = since
				t.lasttx = int64(since)
			}
			t.tot += int64(since)
			break
		}
	}
	if t.txc > 0 {
		log.Info("CUR DURATION OF TX", log.String("cur.duration", minDuration.String()))
		log.Info("AVG DURATION OF TX", log.String("avg.duration", time.Duration(t.tot/t.txc).String()))
	}
	t.mu.Unlock()
}

type loadTracker struct {
	block  *broadcast.Block
	mu     sync.Mutex
	server *node.Server
	total  uint64
	txtt   *txTimeTracker
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
	l.txtt.handleTxTime(round, blocks)
	l.mu.Unlock()
	l.server.Broadcast.Acknowledge(round)
}

func (l *loadTracker) readCounterMore() (uint64, int64, int64, uint64) {
	l.mu.Lock()
	total := l.total
	l.total = 0
	l.txtt.mu.Lock()
	var avgDuration int64
	if l.txtt.txc != 0 {
		avgDuration = l.txtt.tot / l.txtt.txc
	}
	curDuration := l.txtt.lasttx
	curGen := l.txtt.curGen
	l.txtt.mu.Unlock()
	l.mu.Unlock()
	return total, avgDuration, curDuration, curGen
}

func (l *loadTracker) getTotal() uint64 {
	l.mu.Lock()
	total := l.total
	l.mu.Unlock()
	return total
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
	initialRate := opts.Flags("--initial-rate").Label("TXS").Int("Initial number of transactions to send per second [10000]")
	fixedTPS := opts.Flags("--fixed-tps").Label("TXS").Int("Max number of transactions to send per seconds [10000]")
	memProfile := opts.Flags("--mem-profile").Label("PATH").String("Write the memory profile to the given file before exiting")
	rateDecrease := opts.Flags("--rate-decr").Label("FACTOR").Float("Multiplicative decrease factor of the queue size [0.8]")
	rateIncrease := opts.Flags("--rate-incr").Label("FACTOR").Int("Additive increase factor of the queue size [1000]")
	runtimeRoot := opts.Flags("--runtime-root").Label("PATH").String("Path to the runtime root directory [~/.chainspace]", defaultRootDir())
	txSize := opts.Flags("--tx-size").Label("SIZE").Int("Size in bytes of the generated transactions [100]")
	expectedLatency := opts.Flags("--expected-latency").Label("SIZE").Float("Expected latency to deliver transactions in seconds [4]")

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

	nodeCfg.Consensus.RateLimit = &config.RateLimit{
		InitialRate:  *initialRate,
		RateDecrease: *rateDecrease,
		RateIncrease: *rateIncrease,
	}

	nodeCfg.DisableSBAC = true
	nodeCfg.Logging.FileLevel = log.InfoLevel
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
		txtt:   &txTimeTracker{},
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

	fTPS := 0
	if fixedTPS != nil {
		fTPS = *fixedTPS
	}
	if expectedLatency != nil && int(*expectedLatency) != 0 {
		go genLoadWithExpectedLatency(s, app, nodeID, *txSize, *initialRate, *rateIncrease, *rateDecrease, *expectedLatency)
		logMore(app, netCfg.Shard.Size, uint64(*initialRate), uint64(*rateIncrease), *rateDecrease, nodePath)
	} else {
		go genLoad(s, nodeID, *txSize, fTPS)
		logTPS(app, netCfg.Shard.Size, uint64(*initialRate), uint64(*rateIncrease), *rateDecrease)
	}

}

func genLoadWithExpectedLatency(s *node.Server, lt *loadTracker, nodeID uint64, txSize int, initialRate, incRate int, decRate, expectedLatency float64) {
	tt := lt.txtt
	tx := make([]byte, txSize)
	binary.LittleEndian.PutUint32(tx, uint32(nodeID))
	var u uint32
	usize := unsafe.Sizeof(u)
	w := bytes.NewBuffer(tx[usize+usize:])
	gettinHot := true
	currRate := initialRate

	previousCount := 0
	var previousLatency time.Duration

	for {
		rateToDo := currRate
		// pre-heat the load
		if gettinHot || currRate == 0 {
			rateToDo = 1
		}
		log.Info("Generating transactions", log.Int("tx.count", rateToDo))
		// get time before we start sending transaction
		now := time.Now()
		for i := 0; i < rateToDo; i += 1 {
			txnow := []byte(time.Now().Format(time.RFC3339Nano))
			w.Reset()
			binary.LittleEndian.PutUint32(tx[usize:usize+usize], uint32(len(txnow)))
			_, err := w.Write(txnow)
			if err != nil {
				log.Fatal("Could not write in buffer", fld.Err(err))
			}
			s.Broadcast.AddTransaction(tx, 0)
		}
		// get current latency
		tt.mu.Lock()
		latency := time.Duration(tt.lasttx)
		tt.curGen = uint64(rateToDo)
		tt.mu.Unlock()

		// increase or decrease rate
		log.Info("CURRENT LATENCY", log.Uint64("latency", uint64(latency)), log.Float64("lat.sec", latency.Seconds()), log.Float64("epect.lat", expectedLatency))

		// get the current tx per sec
		if previousLatency == latency {
			previousCount += 1
		} else {
			previousCount = 0
		}
		previousLatency = latency

		if previousCount >= 4 {
			currRate = int(float64(currRate) * decRate)
		} else if latency > 0 && latency.Seconds() < expectedLatency {
			if gettinHot == true {
				gettinHot = false
			} else {
				currRate += incRate
			}
		} else if latency.Seconds() > expectedLatency && gettinHot == false {
			currRate = int(float64(currRate) * decRate)
		}

		// we send currRate per sec, we wait until the second is finished
		timeElapsed := time.Since(now)
		log.Info("Waiting before generating more transactions", log.String("wait.duration", time.Duration(time.Second-timeElapsed).String()))
		time.Sleep(time.Second - timeElapsed)
	}

}

func genLoad(s *node.Server, nodeID uint64, txSize int, fixedTPS int) {
	tx := make([]byte, txSize)
	binary.LittleEndian.PutUint32(tx, uint32(nodeID))
	var u uint32
	usize := unsafe.Sizeof(u)
	w := bytes.NewBuffer(tx[usize+usize:])
	if fixedTPS == 0 {
		for {
			now := []byte(time.Now().Format(time.RFC3339Nano))
			w.Reset()
			binary.LittleEndian.PutUint32(tx[usize:usize+usize], uint32(len(now)))
			_, err := w.Write(now)
			if err != nil {
				log.Fatal("Could not write in buffer", fld.Err(err))
			}
			s.Broadcast.AddTransaction(tx, 0)
		}
	} else {
		for {
			for i := 0; i < fixedTPS; i += 1 {
				now := []byte(time.Now().Format(time.RFC3339Nano))
				w.Reset()
				binary.LittleEndian.PutUint32(tx[usize:usize+usize], uint32(len(now)))
				_, err := w.Write(now)
				if err != nil {
					log.Fatal("Could not write in buffer", fld.Err(err))
				}
				s.Broadcast.AddTransaction(tx, 0)
			}
			time.Sleep(time.Second)
		}
	}

}

func logMore(l *loadTracker, nodeCount int, startRate uint64, incr uint64, decr float64, nodePath string) {
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
	recent := make([]float64, 10)
	idx := 0

	f, err := os.Create(filepath.Join(nodePath, "results.csv"))
	if err != nil {
		log.Fatal("unable to create csv file", fld.Err(err))
	}
	csvw := csv.NewWriter(f)
	rec := []string{
		"tps.current",
		"tps.highest",
		"tps.avg",
		"lat.avg",
		"lat.current",
		"tpsgen.current",
		"time.elapsed",
	}
	csvw.Write(rec)
	csvw.Flush()
	starttime := time.Now()
	for {
		<-tick.C
		total, avgLatency, curLatency, curGen := l.readCounterMore()
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
		log.Info("Current Latency", log.Float64("lat.sec", time.Duration(curLatency).Seconds()))

		rec := []string{
			fmt.Sprintf("%v", total),
			fmt.Sprintf("%v", highest),
			fmt.Sprintf("%v", maxTotal/maxCount),
			fmt.Sprintf("%v", time.Duration(avgLatency).Seconds()),
			fmt.Sprintf("%v", time.Duration(curLatency).Seconds()),
			fmt.Sprintf("%v", curGen),
			fmt.Sprintf("%v", time.Since(starttime).String()),
		}
		csvw.Write(rec)
		csvw.Flush()
	}
}

func logTPS(l *loadTracker, nodeCount int, startRate uint64, incr uint64, decr float64) {
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
