package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"chainspace.io/prototype/restsrv"
)

var (
	address  string
	workers  int
	objects  int
	duration int

	metrics = map[int][]time.Duration{}
	mu      sync.Mutex
)

const (
	contractID = "contract_dummy"
	procedure  = "dummy_check_ok"
)

func seedObjects() ([]string, error) {
	out := []string{}
	client := http.Client{}
	url := (&url.URL{
		Scheme: "http",
		Host:   address,
		Path:   "object",
	}).String()
	for i := 0; i < objects; i += 1 {
		payload := bytes.NewBufferString(fmt.Sprintf(`{"data": "%v"}`, randSeq(30)))
		req, err := http.NewRequest(http.MethodPost, url, payload)
		req.Header.Add("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error calling node: %v", err.Error())
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response from POST /object: %v", err.Error())
		}
		resp.Body.Close()
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

func makeTransactionPayload(seed []string) []byte {
	outputs := []string{}
	for i := 0; i < objects; i += 1 {
		outputs = append(outputs, randSeq(30))
	}
	tx := restsrv.Transaction{
		Traces: []restsrv.Trace{
			{
				ContractID:       contractID,
				Procedure:        procedure,
				InputObjectsKeys: seed,
				OutputObjects:    outputs,
			},
		},
	}
	txbytes, _ := json.Marshal(tx)
	return txbytes
}

func worker(ctx context.Context, seed []string, wg *sync.WaitGroup, id int) {
	wg.Add(1)
	defer wg.Done()

	client := http.Client{}
	url := (&url.URL{
		Scheme: "http",
		Host:   address,
		Path:   "transaction",
	}).String()

	mu.Lock()
	metrics[id] = []time.Duration{}
	mu.Unlock()

	for {
		if err := ctx.Err(); err != nil {
			return
		}

		fmt.Printf("worker %v sending new transaction\n", id)
		start := time.Now()
		// make transaction
		txbytes := makeTransactionPayload(seed)
		payload := bytes.NewBuffer(txbytes)
		req, err := http.NewRequest(http.MethodPost, url, payload)
		req.Header.Add("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("error calling node: %v", err.Error())
			os.Exit(1)
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("error reading response from POST /object: %v", err.Error())
			os.Exit(1)
		}
		resp.Body.Close()

		res := struct {
			Data   []restsrv.Object `json:"data"`
			Status string           `json:"status"`
		}{}
		err = json.Unmarshal(b, &res)
		if err != nil {
			fmt.Printf("unable to unmarshal response: %v\n", err.Error())
			os.Exit(1)
		}
		if res.Status != "success" {
			fmt.Printf("error from server: %v\n", string(b))
			os.Exit(1)
		}

		for i, v := range res.Data {
			seed[i] = v.Key
		}

		mu.Lock()
		metrics[id] = append(metrics[id], time.Since(start))
		mu.Unlock()
		fmt.Printf("worker %v received new objects keys: %v\n", id, seed)
	}
}

func main() {
	if len(address) <= 0 {
		fmt.Println("missing address parameter")
		os.Exit(1)
	}
	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	for i := 0; i < workers; i += 1 {
		seeds, err := seedObjects()
		if err != nil {
			fmt.Println(err.Error())
			cancel()
			wg.Wait()
			return
		}
		fmt.Printf("starting worker %v\n", i)
		go worker(ctx, seeds, wg, i)
	}

	t := time.NewTimer(time.Duration(duration) * time.Second)
	fmt.Printf("starting test for %v seconds (until %v)\n", time.Duration(duration)*time.Second, time.Now().Add(time.Duration(duration)*time.Second).Format(time.Stamp))
	select {
	case <-t.C:
		cancel()
		wg.Wait()
	}
	durations := []time.Duration{}
	for k, v := range metrics {
		v := v
		fmt.Printf("worker %v: %v txs sent", k, len(v))
		durations = append(durations, v...)
	}
	var total uint64
	for _, v := range durations {
		total += uint64(v)
	}
	fmt.Printf("average duration per tx: %v\n", time.Duration(total/uint64(len(durations))))

	fmt.Println("looks good to me.")
}

func init() {
	flag.StringVar(&address, "addr", "", "address of the node http server to use")
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
