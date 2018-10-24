package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type testrunner struct {
	workerspool [][]*worker
	wg          *sync.WaitGroup
	cancel      func()
	workercount int
}

func (tr *testrunner) getReadyWorkers() []*worker {
	for _, workrs := range tr.workerspool {
		workrs := workrs
		ok := true
		for _, workr := range workrs {
			workr := workr
			if !workr.Ready() {
				ok = false
				continue
			}
		}
		if ok == true {
			return workrs
		}
	}
	return nil
}

func (tr *testrunner) initWorkers() []*worker {
	workrs := []*worker{}
	for i := 0; i < workers; i += 1 {
		tr.workercount += 1
		seeds, err := seedObjects(tr.workercount)
		for err != nil {
			fmt.Println(err.Error())
			seeds, err = seedObjects(tr.workercount)
		}
		fmt.Printf("seeds generated successfully\n")
		labels := makeLabels(tr.workercount)
		fmt.Printf("starting worker %v\n", tr.workercount)
		w := NewWorker(seeds, labels, tr.workercount)
		workrs = append(workrs, w)

	}

	return workrs
}

func (tr *testrunner) runWorkers(ctx context.Context, workrs []*worker) {
	for _, workr := range workrs {
		workr := workr
		go workr.run(ctx, tr.wg)
	}
}

func (tr *testrunner) Run(ctx context.Context, cancel func()) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			now := time.Now()

			workrs := tr.getReadyWorkers()
			if workrs != nil {
				tr.runWorkers(ctx, workrs)
			} else {
				workrs = tr.initWorkers()
				tr.workerspool = append(tr.workerspool, workrs)
				tr.runWorkers(ctx, workrs)
			}

			timeelapsed := time.Since(now)
			waitfor := time.Duration(time.Second - timeelapsed)
			fmt.Printf("waiting before sending more transation: %v\n", waitfor)
			time.Sleep(waitfor)
		}

	}
}

func NewTestRunner(wg *sync.WaitGroup) *testrunner {
	return &testrunner{
		workerspool: [][]*worker{},
		wg:          wg,
	}
}
