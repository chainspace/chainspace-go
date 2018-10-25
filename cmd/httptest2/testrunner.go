package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type testrunner struct {
	workerspool []*worker
	wg          *sync.WaitGroup
	cancel      func()
	workercount int
	mu          sync.Mutex
}

func (tr *testrunner) getReadyWorkers() []*worker {
	out := []*worker{}
	for _, workr := range tr.workerspool {
		workr := workr
		if workr.Ready() {
			out = append(out, workr)
		}
		if len(out) >= workers {
			return out
		}
	}

	for len(out) < workers {
		wrkr := tr.initWorker()
		out = append(out, wrkr)
	}

	return out
}

func (tr *testrunner) initWorker() *worker {
	tr.mu.Lock()
	tr.workercount += 1
	wrkrcount := tr.workercount
	tr.mu.Unlock()
	seeds, err := seedObjects(wrkrcount)
	for err != nil {
		fmt.Println(err.Error())
		seeds, err = seedObjects(wrkrcount)
	}
	fmt.Printf("seeds generated successfully\n")
	labels := makeLabels(wrkrcount)
	fmt.Printf("starting worker %v\n", wrkrcount)
	w := NewWorker(seeds, labels, wrkrcount)
	tr.mu.Lock()
	tr.workerspool = append(tr.workerspool, w)
	tr.mu.Unlock()
	return w
}

func (tr *testrunner) runWorkers(ctx context.Context, workrs []*worker) {
	waitfor := time.Second / time.Duration(workers)
	for _, workr := range workrs {
		workr := workr
		go workr.run(ctx, tr.wg)
		time.Sleep(waitfor)
	}
}

func (tr *testrunner) Run(ctx context.Context, cancel func()) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			workrs := tr.getReadyWorkers()
			tr.runWorkers(ctx, workrs)
		}

	}
}

func (tr *testrunner) InitWorkers() {
	wg := &sync.WaitGroup{}
	for i := 0; i < workers*10; i += 1 {
		wg.Add(1)
		go func(w *sync.WaitGroup) {
			defer w.Done()
			tr.initWorker()
		}(wg)
	}
	wg.Wait()
}

func NewTestRunner(wg *sync.WaitGroup) *testrunner {
	return &testrunner{
		workerspool: []*worker{},
		wg:          wg,
	}
}
