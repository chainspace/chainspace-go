package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"

	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
)

var (
	address     string
	port        int
	workers     int
	objects     int
	duration    int
	subPort     int
	networkName string
	nodeCount   int
	shardCount  int

	metrics = map[int][]time.Duration{}
	mu      sync.Mutex

	addresses       []string
	pubsubAddresses = map[uint64]string{}

	subscribr *subscriber
)

const (
	contractID = "dummy"
	procedure  = "dummy_ok"
)

type outputty struct {
	Label []string `json:"label"`
	Value string   `json:"value"`
}

func init() {
	flag.StringVar(&address, "addr", "", "address of the node http server to use")
	flag.IntVar(&port, "port", 0, "port to connect in order to send transactions (use with gcp)")
	flag.IntVar(&workers, "txs", 1, "number of transactions to send per seconds default=1")
	flag.IntVar(&objects, "objects", 1, "number of objects to be used as inputs in the transaction default=1")
	flag.IntVar(&duration, "duration", 5, "duration of the test")
	flag.IntVar(&subPort, "pubsub-port", 0, "pubsub port, this is the same port for all nodes")
	flag.StringVar(&networkName, "network-name", "testnet", "network name")
	flag.IntVar(&nodeCount, "node-count", 4, "node count per shard")
	flag.IntVar(&shardCount, "shard-count", 1, "node count per shard")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
}

func seedObjects(id int) ([]string, error) {
	out := []string{}
	_addr := getAddress(id)
	url := (&url.URL{
		Scheme: "http",
		Host:   _addr,
		Path:   "object",
	}).String()
	fmt.Printf("seeding objects for worker %v with %v\n", id, _addr)
	for i := 0; i < objects; i += 1 {
		client := http.Client{
			Timeout: 15 * time.Second,
		}
		payload := bytes.NewBufferString(fmt.Sprintf(`{"data": "%v"}`, randSeq(30)))
		req, err := http.NewRequest(http.MethodPost, url, payload)
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("X-request-start", time.Now().Format(time.RFC3339Nano))
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error calling node: %v", err.Error())
		}
		b, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("error reading response from POST /object: %v", err.Error())
		}
		res := struct {
			Data   interface{} `json:"data"`
			Status string      `json:"status"`
		}{}
		err = json.Unmarshal(b, &res)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal response: %v", err.Error())
		}
		if res.Status != "success" {
			return nil, fmt.Errorf("error from server: %v", res.Data)
		}
		key := res.Data.(map[string]interface{})["id"].(string)
		out = append(out, key)
		fmt.Printf("new seed key: %v\n", key)
	}
	return out, nil
}

func main() {
	if len(address) <= 0 && port <= 0 {
		fmt.Println("missing address or port parameter")
		os.Exit(1)
	}
	if port > 0 && len(address) > 0 {
		getAddresses()
	} else if port > 0 {
		getAddressesFromGCP()
	}
	ctx, cancel := context.WithCancel(context.Background())

	wg := &sync.WaitGroup{}
	tr := NewTestRunner(wg)
	tr.InitWorkers()

	t := time.NewTimer(time.Duration(duration) * time.Second)
	fmt.Printf("starting tests (at %v) for %v seconds (until %v)\n",
		time.Now(), time.Duration(duration)*time.Second,
		time.Now().Add(time.Duration(duration)*time.Second).Format(time.Stamp))

	// run testrunner
	go tr.Run(ctx, cancel)
	select {
	case <-t.C:
		cancel()
		wg.Wait()
	}
	durations := []time.Duration{}
	txcounts := orderResults()
	var totaltx int
	for _, v := range txcounts {
		v := v
		fmt.Printf("worker %v: %v txs\n", v.id, v.count)
		totaltx += v.count
	}
	for _, v := range metrics {
		v := v
		durations = append(durations, v...)
	}
	var total uint64
	for _, v := range durations {
		total += uint64(v)
	}
	fmt.Printf("total txs: %v\n", totaltx)
	fmt.Printf("average duration per tx: %v\n", time.Duration(total/uint64(len(durations))))
	fmt.Printf("testing finished at: %v\n", time.Now().Format(time.Stamp))
	fmt.Printf("average tx per secs: %v\n", totaltx/duration)
}

type WorkerTxCount struct {
	id    int
	count int
}

func orderResults() []WorkerTxCount {
	out := []WorkerTxCount{}
	for k, v := range metrics {
		out = append(out, WorkerTxCount{k, len(v)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

func makeLabels(workerID int) [][]string {
	labels := [][]string{}
	for i := 0; i < objects; i += 1 {
		l := fmt.Sprintf("label:%v:%v", workerID, i)
		fmt.Printf("creating new object with label: %v\n", l)
		labels = append(labels, []string{l})
	}
	return labels
}

func getAddressesFromGCP() {
	ctx := context.Background()

	c, err := google.DefaultClient(ctx, compute.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}

	computeService, err := compute.New(c)
	if err != nil {
		log.Fatal(err)
	}

	// Project ID for this request.
	project := "acoustic-atom-211511" // TODO: Update placeholder value.

	req := computeService.Instances.List(project, "europe-west2-b")
	if err := req.Pages(ctx, func(page *compute.InstanceList) error {
		for n, v := range page.Items {
			addresses = append(addresses, v.NetworkInterfaces[0].AccessConfigs[0].NatIP+":"+fmt.Sprintf("%v", port))
			pubsubAddresses[uint64(n)] = v.NetworkInterfaces[0].AccessConfigs[0].NatIP + ":" + fmt.Sprintf("%v", subPort)
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}
}

func getAddresses() {
	for i := 1; i <= (nodeCount * shardCount); i += 1 {
		addresses = append(addresses, fmt.Sprintf("%v:%v", address, port+i))
	}
}

func getAddress(workerID int) string {
	return addresses[workerID%len(addresses)]
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return base64.StdEncoding.EncodeToString([]byte(string(b)))
}
