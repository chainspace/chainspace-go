package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"chainspace.io/prototype/broadcast"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/node"
	"github.com/gogo/protobuf/proto"
)

var (
	configRoot  string
	runtimeRoot string
	networkName string
	outdir      string
	nodeID      uint64
	txSize      int
	tps         int
	duration    time.Duration
)

func init() {
	flag.StringVar(&configRoot, "config-root", defaultRootDir(), "Path to the chainspace root directory")
	flag.StringVar(&runtimeRoot, "runtime-root", defaultRootDir(), "Path to the runtime root directory")
	flag.StringVar(&networkName, "network", "", "Name of the network to use")
	flag.StringVar(&outdir, "outdir", path.Join(defaultRootDir(), "testresults"), "Folder for the output test results")
	flag.Uint64Var(&nodeID, "nodeid", 0, "Node id to start")
	flag.IntVar(&txSize, "tx-size", 100, "Size in bytes of the generated transaction")
	flag.IntVar(&tps, "tps", 1000, "Maximum number of transaction generated per seconds")
	flag.DurationVar(&duration, "duration", 60*time.Second, "Duration of the test, valid time units are s, m, h")
	flag.Parse()
}

type txTimeTracker struct {
	mu     sync.Mutex
	lasttx int64
	tot    int64
	txc    int64

	curGen uint64

	durations  []time.Duration
	avgLatency time.Duration
}

func (t *txTimeTracker) handleTxTime(
	round uint64, blocks []*broadcast.SignedData) {
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
			t.txc++
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
		t.durations = append(t.durations, minDuration)
		t.avgLatency = time.Duration(t.tot / t.txc)
	}
	t.mu.Unlock()
}

type testrunner struct {
	block   *broadcast.Block
	mu      sync.Mutex
	srv     *node.Server
	total   uint64
	txtt    *txTimeTracker
	outfile string
	avgTxs  uint64
}

func (tr *testrunner) handleDeliver(round uint64, blocks []*broadcast.SignedData) {
	tr.mu.Lock()
	block := tr.block
	for _, signed := range blocks {
		block.Transactions.Count = 0
		block.Transactions.Data = nil
		if err := proto.Unmarshal(signed.Data, block); err != nil {
			log.Fatal("Unable to decode delivered signed block", fld.Err(err))
		}
		tr.total += block.Transactions.Count
	}
	tr.txtt.handleTxTime(round, blocks)
	tr.mu.Unlock()
	tr.srv.Broadcast.Acknowledge(round)
}

func (tr *testrunner) getTotal() uint64 {
	tr.mu.Lock()
	total := tr.total
	tr.mu.Unlock()
	return total
}

func (tr *testrunner) readCounter() uint64 {
	tr.mu.Lock()
	total := tr.total
	tr.total = 0
	tr.mu.Unlock()
	return total
}

func (tr *testrunner) calcTxPerSecs(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	tick := time.NewTicker(time.Second)
	var (
		total uint64
		count uint64
	)
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		<-tick.C
		total += uint64(tr.readCounter())
		count++
		if total != 0 {
			tr.avgTxs = total / count
		}
	}
}

type testresults struct {
	AvgLatency float64 `json:"avgLatency"`
	AvgTxs     uint64  `json:"avgTxs"`
}

func (tr *testrunner) writeResults() {
	tres := testresults{
		AvgLatency: tr.txtt.avgLatency.Seconds(),
		AvgTxs:     tr.avgTxs,
	}
	b, err := json.MarshalIndent(&tres, "", "  ")
	if err != nil {
		fmt.Printf("unable to marshal test results: %v\n", err)
		os.Exit(1)
	}
	err = ioutil.WriteFile(tr.outfile, b, 0644)
	if err != nil {
		fmt.Printf("unable to write test results to file: %v\n", err)
		os.Exit(1)
	}
}

func genLoad(
	ctx context.Context,
	s *node.Server,
	nodeID uint64,
	txSize int,
	expectedtps int,
) {
	tx := make([]byte, txSize)
	binary.LittleEndian.PutUint32(tx, uint32(nodeID))
	var u uint32
	usize := unsafe.Sizeof(u)
	w := bytes.NewBuffer(tx[usize+usize:])
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		for i := 0; i < expectedtps; i += 1 {
			now := []byte(time.Now().Format(time.RFC3339Nano))
			w.Reset()
			binary.LittleEndian.PutUint32(tx[usize:usize+usize], uint32(len(now)))
			_, err := w.Write(now)
			if err != nil {
				fmt.Printf("Could not write in buffer: %v\n", err)
				os.Exit(1)
			}
			s.Broadcast.AddTransaction(tx, 0)
		}
		time.Sleep(time.Second)
	}
}

func defaultRootDir() string {
	return os.ExpandEnv("$HOME/.chainspace")
}

func ensureDir(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Mkdir(path, 0700)
		}
		return err
	}
	return nil
}

func main() {
	var haserr bool
	if len(networkName) <= 0 {
		fmt.Printf("missing mandatory argument `network`\n")
		haserr = true
	}
	if nodeID <= 0 {
		fmt.Printf("missing mandatory argument `nodeid`\n")
		haserr = true
	}
	if haserr == true {
		flag.Usage()
		os.Exit(1)
	}

	if err := ensureDir(outdir); err != nil {
		fmt.Printf("unable to create results output dir: %v\n", err)
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

	s, err := node.Run(cfg)
	if err != nil {
		log.Fatal("Could not start node", fld.NodeID(nodeID), fld.Err(err))
	}

	tr := &testrunner{
		block: &broadcast.Block{
			Transactions: &broadcast.Transactions{},
		},
		srv:  s,
		txtt: &txTimeTracker{},
		outfile: path.Join(
			outdir,
			fmt.Sprintf("blockmaniatest-results-node%v.json", nodeID)),
	}
	s.Broadcast.Register(tr.handleDeliver)

	ctx, _ := context.WithTimeout(context.Background(), duration)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go tr.calcTxPerSecs(ctx, &wg)

	go genLoad(ctx, s, nodeID, txSize, tps)
	// wait for the context to exit
	<-ctx.Done()
	s.Shutdown()

	// wait for calcTxPerSecs to finish
	wg.Wait()

	//create the results
	tr.writeResults()
}
