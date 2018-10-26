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

	"chainspace.io/prototype/restsrv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

var (
	address  string
	port     string
	workers  int
	objects  int
	duration int

	metrics = map[int][]time.Duration{}
	mu      sync.Mutex

	addresses []string
)

const (
	contractID = "dummy"
	procedure  = "dummy_ok"
)

func getAddress(workerID int) string {
	if len(address) > 0 {
		return address
	}
	return addresses[workerID%len(addresses)]
}

type seedType struct {
	Data interface{} `json:"data"`
}

type outputty struct {
	Label []string `json:"label"`
	Value string   `json:"value"`
}

func seedObjects(i int) ([]string, error) {
	out := []string{}
	_addr := getAddress(i)
	url := (&url.URL{
		Scheme: "http",
		Host:   _addr,
		Path:   "object",
	}).String()
	_id := i
	fmt.Printf("seeding objects for worker %v with %v\n", i, _addr)
	for i := 0; i < objects; i += 1 {
		client := http.Client{
			Timeout: 15 * time.Second,
		}

		t := seedType{
			outputty{
				[]string{fmt.Sprintf("label:%v:%v", _id, i)},
				"seed:0:0",
			},
		}
		bt, _ := json.Marshal(&t)
		payload := bytes.NewBufferString(string(bt))
		req, err := http.NewRequest(http.MethodPost, url, payload)
		req.Header.Add("Content-Type", "application/json")
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

func objectsReady(ctx context.Context, workerID int, seed []string) {
	url := (&url.URL{
		Scheme: "http",
		Host:   getAddress(workerID),
		Path:   "object/ready",
	}).String()

	for {
		if err := ctx.Err(); err != nil {
			fmt.Printf("context error: %v\n", err)
			break
		}
		time.Sleep(time.Second)
		client := http.Client{
			Timeout: 15 * time.Second,
		}
		p := struct {
			Data []string `json:"data"`
		}{
			Data: seed,
		}
		by, _ := json.Marshal(&p)
		payload := bytes.NewBuffer(by)
		req, err := http.NewRequest(http.MethodPost, url, payload)
		if err != nil {
			fmt.Printf("unable to create request")
			continue
		}
		req = req.WithContext(ctx)
		req.Header.Add("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("error calling node from objectReady: %v\n", err.Error())
			continue
		}
		b, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Printf("error reading response from POST /object/ready: %v\n", err.Error())
			continue
		}
		res := struct {
			Data   bool   `json:"data"`
			Status string `json:"status"`
		}{}
		err = json.Unmarshal(b, &res)
		if err != nil {
			fmt.Printf("unable to unmarshal response worker=%v err=%v res=%v\n", workerID, err.Error(), string(b))
			continue
		}
		if res.Status != "success" {
			fmt.Printf("error from server: worker=%v err=%v\n", workerID, string(b))
			continue
		}
		if res.Data == true {
			fmt.Printf("worker %v all objects created and queryable with success\n", workerID)
			return
		}
		// fmt.Printf("worker %v some object still unavailable\n", workerID)
	}
}

func makeTransactionPayload(seed []string, objectsData []interface{}, labels [][]string, iter int) []byte {
	outputs := []interface{}{}
	for i := 0; i < objects; i += 1 {
		outputs = append(outputs, outputty{labels[i], fmt.Sprintf("worker:%v:%v", i, iter)})
	}
	mappings := map[string]interface{}{}
	for i, _ := range objectsData {
		mappings[seed[i]] = objectsData[i]
	}
	tx := restsrv.Transaction{
		Traces: []restsrv.Trace{
			{
				ContractID:            contractID,
				Procedure:             procedure,
				InputObjectVersionIDs: seed,
				OutputObjects:         outputs,
				Labels:                labels,
			},
		},
		Mappings: mappings,
	}
	txbytes, _ := json.Marshal(tx)
	return txbytes
}

func worker(ctx context.Context, seed []string, labels [][]string, wg *sync.WaitGroup, id int) {
	/*
		seed, err := seedObjects()
		if err != nil {
			fmt.Printf("unable to create seed object: %v\n", err)
		}
	*/
	wg.Add(1)
	defer wg.Done()

	url := (&url.URL{
		Scheme: "http",
		Host:   getAddress(id),
		Path:   "transaction",
	}).String()

	mu.Lock()
	metrics[id] = []time.Duration{}
	mu.Unlock()

	// empty objects first
	objectsData := []interface{}{}
	for _, _ = range seed {
		objectsData = append(objectsData, map[string]interface{}{})
	}
	i := 0
	for {
		if err := ctx.Err(); err != nil {
			fmt.Printf("context error: %v\n", err)
			break
		}

		client := http.Client{
			Timeout: 15 * time.Second,
		}

		fmt.Printf("worker %v sending new transaction\n", id)
		start := time.Now()
		// make transaction
		txbytes := makeTransactionPayload(seed, objectsData, labels, i)
		payload := bytes.NewBuffer(txbytes)
		req, err := http.NewRequest(http.MethodPost, url, payload)
		req = req.WithContext(ctx)
		req.Header.Add("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("error calling node from worker: %v\n", err.Error())
			continue
		}
		b, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Printf("error reading response from POST /object: %v\n", err.Error())
			continue
		}

		res := struct {
			Data   interface{} `json:"data"`
			Status string      `json:"status"`
		}{}
		err = json.Unmarshal(b, &res)
		if err != nil {
			fmt.Printf("unable to unmarshal response: %v\n", err.Error())
			break
		}
		if res.Status != "success" {
			fmt.Printf("error from server: %v\n", string(b))
			continue
		}

		data := res.Data.([]interface{})
		objectsData = []interface{}{}
		for i, v := range data {
			seed[i] = v.(map[string]interface{})["key"].(string)
			objectsData = append(
				objectsData, v.(map[string]interface{})["value"].(interface{}))
		}

		mu.Lock()
		metrics[id] = append(metrics[id], time.Since(start))
		tot := len(metrics[id])
		mu.Unlock()
		// ensure objects are all ready baby
		objectsReady(ctx, id, seed)
		// fmt.Printf("worker %v received new objects keys: %v\n", id, seed)
		fmt.Printf("worker %v received new objects keys: %v\n", id, tot)
		// time.Sleep(2 * time.Second)
		i += 1
	}
	fmt.Printf("worker %v finished at: %v\n", id, time.Now().Format(time.Stamp))
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
		for _, v := range page.Items {
			addresses = append(addresses, v.NetworkInterfaces[0].AccessConfigs[0].NatIP+":"+port)
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}
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

func main() {
	if len(address) <= 0 && len(port) <= 0 {
		fmt.Println("missing address or port parameter")
		os.Exit(1)
	}
	if len(address) > 0 && len(port) > 0 {
		fmt.Println("cannot specify address and port parameter together")
		os.Exit(1)
	}
	if len(port) > 0 {
		getAddressesFromGCP()
	}

	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	seeds := make([][]string, workers)
	// create seeds
	labels := make([][][]string, workers)
	for i := 0; i < workers; i += 1 {
		s, err := seedObjects(i)
		if err != nil {
			fmt.Println(err.Error())
			cancel()
			wg.Wait()
			return
		}
		seeds[i] = s
		labels[i] = makeLabels(i)
	}

	fmt.Printf("seeds generated successfully\n")
	t := time.NewTimer(time.Duration(duration) * time.Second)
	// start txs
	for i := 0; i < workers; i += 1 {
		fmt.Printf("starting worker %v\n", i)
		go worker(ctx, seeds[i], labels[i], wg, i)
	}

	fmt.Printf("starting tests (at %v) for %v seconds (until %v)\n", time.Now(), time.Duration(duration)*time.Second, time.Now().Add(time.Duration(duration)*time.Second).Format(time.Stamp))
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

func init() {
	flag.StringVar(&address, "addr", "", "address of the node http server to use")
	flag.StringVar(&port, "port", "", "port to connect in order to send transactions (use with gcp)")
	flag.IntVar(&workers, "workers", 1, "number of workers used to send transactions default=1")
	flag.IntVar(&objects, "objects", 1, "number of objects to be used as inputs in the transaction default=1")
	flag.IntVar(&duration, "duration", 5, "duration of the test")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return base64.StdEncoding.EncodeToString([]byte(string(b)))
}
