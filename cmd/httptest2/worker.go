package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"chainspace.io/prototype/restsrv"
)

type worker struct {
	seed       []string
	labels     [][]string
	id         int
	url        string
	notify     chan struct{}
	pendingIDs map[string]struct{}
}

func NewWorker(seed []string, labels [][]string, id int) *worker {
	url := (&url.URL{
		Scheme: "http",
		Host:   getAddress(id),
		Path:   "transaction",
	}).String()

	return &worker{
		seed:       seed,
		labels:     labels,
		id:         id,
		url:        url,
		notify:     make(chan struct{}),
		pendingIDs: map[string]struct{}{},
	}
}

func makeTransactionPayload(seed []string, labels [][]string) []byte {
	outputs := []interface{}{}
	for i := 0; i < objects; i += 1 {
		outputs = append(outputs, outputty{labels[i], randSeq(30)})
	}
	tx := restsrv.Transaction{
		Traces: []restsrv.Trace{
			{
				ContractID:       contractID,
				Procedure:        procedure,
				InputObjectsKeys: seed,
				OutputObjects:    outputs,
				Labels:           labels,
			},
		},
	}
	txbytes, _ := json.Marshal(tx)
	return txbytes
}

func (w *worker) cb(objectID string) {
	delete(w.pendingIDs, objectID)
	if len(w.pendingIDs) <= 0 {
		w.notify <- struct{}{}
	}
}

func (w *worker) run(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	mu.Lock()
	metrics[w.id] = []time.Duration{}
	mu.Unlock()

	for {
		if err := ctx.Err(); err != nil {
			fmt.Printf("context error: %v\n", err)
			break
		}

		client := http.Client{
			Timeout: 5 * time.Second,
		}
		fmt.Printf("worker %v sending new transaction\n", w.id)
		start := time.Now()
		// make transaction
		txbytes := makeTransactionPayload(w.seed, w.labels)
		payload := bytes.NewBuffer(txbytes)
		req, err := http.NewRequest(http.MethodPost, w.url, payload)
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
		for i, v := range data {
			w.seed[i] = v.(map[string]interface{})["key"].(string)
			subscribr.Subscribe(w.seed[i], w.cb)
			w.pendingIDs[w.seed[i]] = struct{}{}
		}

		// bock while waiting to get notified
		select {
		case <-w.notify:
			mu.Lock()
			metrics[w.id] = append(metrics[w.id], time.Since(start))
			tot := len(metrics[w.id])
			mu.Unlock()
			fmt.Printf("worker %v all objects created: %v\n", w.id, tot)
			continue
		case <-ctx.Done():
			return
		}

	}
	fmt.Printf("worker %v finished at: %v\n", w.id, time.Now().Format(time.Stamp))
}
