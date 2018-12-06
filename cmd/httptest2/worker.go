package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	checkerapi "chainspace.io/prototype/checker/api"
	sbacapi "chainspace.io/prototype/sbac/api"
)

type worker struct {
	seed       []string
	labels     [][]string
	id         int
	url        string
	notify     chan struct{}
	pendingIDs map[string]struct{}
	ready      bool
	mu         sync.Mutex
	objsdata   []interface{}
}

func NewWorker(seed []string, labels [][]string, id int) *worker {
	var u string
	addr := getAddress(id)
	if standaloneCheckers {
		u = (&url.URL{
			Scheme: "http",
			Host:   addr,
			Path:   "/api/sbac/transaction/checked",
		}).String()
	} else {
		u = (&url.URL{
			Scheme: "http",
			Host:   addr,
			Path:   "/api/sbac/transaction",
		}).String()
	}

	objsdata := []interface{}{}
	for _, _ = range seed {
		objsdata = append(objsdata, map[string]interface{}{})
	}

	return &worker{
		seed:       seed,
		labels:     labels,
		id:         id,
		url:        u,
		notify:     make(chan struct{}),
		pendingIDs: map[string]struct{}{},
		ready:      true,
		objsdata:   objsdata,
	}
}

func (w *worker) callChecker(
	ctx context.Context, addr string, txbytes []byte) (uint64, string, error) {
	u := (&url.URL{
		Scheme: "http",
		Host:   addr,
		Path:   "api/checker/check",
	}).String()
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	fmt.Printf("worker %v checking new transaction with %v\n", w.id, u)
	// make transaction
	payload := bytes.NewBuffer(txbytes)
	req, err := http.NewRequest(http.MethodPost, u, payload)
	req = req.WithContext(ctx)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("error calling checker from worker: %v\n", err.Error())
		return 0, "error", err
	}
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		fmt.Printf("error reading response from POST /object: %v\n", err.Error())
		return 0, "error", err
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("%v\n", string(b))
		return 0, "error", errors.New("error")
	}

	res := checkerapi.CheckTransactionResponse{}
	err = json.Unmarshal(b, &res)
	if err != nil {
		fmt.Printf("error checking transaction (%v)\n", err)
		return 0, "error", err
	}

	return res.NodeID, res.Signature, nil
}

func (w *worker) checkTransaction(ctx context.Context, tx []byte) map[uint64]string {
	signatures := map[uint64]string{}
	wg := sync.WaitGroup{}
	for _, addr := range checkerAddresses {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			for {
				if id, sig, err := w.callChecker(ctx, addr, tx); err == nil {
					mu.Lock()
					fmt.Printf("worker(%v) new signature from node(%v) %v\n", w.id, id, sig)
					signatures[id] = sig
					mu.Unlock()
					return
				}
				if err := ctx.Err(); err != nil {
					fmt.Printf("checker context error: %v\n", err)
					return
				}
			}
		}(addr)
	}
	wg.Wait()
	return signatures
}

func (w *worker) makeTransactionPayload(
	ctx context.Context, seed []string, labels [][]string, objsdata []interface{}) []byte {
	outputs := []sbacapi.OutputObject{}
	for i := 0; i < objects; i += 1 {
		obj := outputty{labels[i], randSeq(30)}
		b, _ := json.Marshal(&obj)
		out := sbacapi.OutputObject{
			Labels: labels[i],
			Object: string(b),
		}
		outputs = append(outputs, out)
	}
	mappings := map[string]interface{}{}
	for i, _ := range objsdata {
		mappings[seed[i]] = objsdata[i]
	}
	tx := sbacapi.Transaction{
		Traces: []sbacapi.Trace{
			{
				ContractID:            contractID,
				Procedure:             procedure,
				InputObjectVersionIDs: seed,
				OutputObjects:         outputs,
			},
		},
		Mappings: mappings,
	}
	txbytes, _ := json.Marshal(tx)
	sigs := w.checkTransaction(ctx, txbytes)
	tx.Signatures = sigs
	txbytes, _ = json.Marshal(tx)
	return txbytes
}

func (w *worker) Ready() bool {
	w.mu.Lock()
	ret := w.ready
	w.mu.Unlock()
	return ret
}

func (w *worker) setReady(ready bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ready = ready
}

func (w *worker) cb(objectID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.pendingIDs, objectID)
	if len(w.pendingIDs) <= 0 {
		w.notify <- struct{}{}
	}
	w.ready = true
}

func (w *worker) run(ctx context.Context, wg *sync.WaitGroup) {
	w.setReady(false)
	wg.Add(1)
	defer wg.Done()

	mu.Lock()
	if _, ok := metrics[w.id]; !ok {
		metrics[w.id] = []time.Duration{}
	}
	mu.Unlock()

	if err := ctx.Err(); err != nil {
		fmt.Printf("context error: %v\n", err)
		return
	}

	client := http.Client{
		Timeout: 5 * time.Second,
	}
	fmt.Printf("worker %v sending new transaction\n", w.id)
	start := time.Now()
	// make transaction
	txbytes := w.makeTransactionPayload(ctx, w.seed, w.labels, w.objsdata)
	payload := bytes.NewBuffer(txbytes)
	req, err := http.NewRequest(http.MethodPost, w.url, payload)
	req = req.WithContext(ctx)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		w.setReady(true)
		fmt.Printf("error calling node from worker: %v\n", err.Error())
		return
	}
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("%v", string(b))
		w.setReady(true)
		return
	}
	if err != nil {
		w.setReady(true)
		fmt.Printf("error reading response from POST /object: %v\n", err.Error())
		return
	}

	res := sbacapi.ObjectResponse{}
	err = json.Unmarshal(b, &res)
	if err != nil {
		w.setReady(true)
		fmt.Printf("unable to unmarshal response: %v\n", err.Error())
		return
	}

	data := res.Object.([]interface{})
	w.objsdata = []interface{}{}
	for i, v := range data {
		w.seed[i] = v.(map[string]interface{})["versionId"].(string)
		w.pendingIDs[w.seed[i]] = struct{}{}
		subscribr.Subscribe(w.seed[i], w.cb)
		w.objsdata = append(
			w.objsdata, v.(map[string]interface{})["value"].(interface{}))
	}

	// bock while waiting to get notified
	select {
	case <-w.notify:
		mu.Lock()
		metrics[w.id] = append(metrics[w.id], time.Since(start))
		tot := len(metrics[w.id])
		mu.Unlock()
		fmt.Printf("worker %v all objects created: %v\n", w.id, tot)
	case <-ctx.Done():
		return
	}
}
