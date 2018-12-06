package checker // import "chainspace.io/prototype/checker"

import (
	"context"
	"errors"
	fmt "fmt"

	sbac "chainspace.io/prototype/sbac"
	"golang.org/x/sync/errgroup"
)

// Checker define the interface for a contract procedure check
type Checker interface {
	ContractID() string
	Name() string
	Check(inputs, refInputs, parameters [][]byte, outputObjects []*sbac.OutputObject, returns [][]byte, dependencies []*sbac.Trace) bool
}

// a pair of a trace and it's associated checker to be store in a slice
type checkerTracePair struct {
	Checker Checker
	Trace   *sbac.Trace
}

// aggregateCheckers first ensure that all the contracts and procedures used in the transaction
// exists then map each transaction to the associated contract.
func aggregate(checkers checkersMap, traces []*sbac.Trace) ([]checkerTracePair, error) {
	var ok bool
	var m map[string]Checker
	var checker Checker
	ctpairs := []checkerTracePair{}
	for _, t := range traces {
		m, ok = checkers[t.ContractID]
		if !ok {
			return nil, fmt.Errorf("unknown contract with ID: %v", t.ContractID)
		}
		checker, ok = m[t.Procedure]
		if !ok {
			return nil, fmt.Errorf("unknown procedure %v for contract with ID: %v", t.Procedure, t.ContractID)
		}
		newpairs, err := aggregate(checkers, t.Dependencies)
		if err != nil {
			return nil, err
		}
		newpair := checkerTracePair{Checker: checker, Trace: t}
		ctpairs = append(ctpairs, newpair)
		ctpairs = append(ctpairs, newpairs...)
	}
	return ctpairs, nil
}

func run(ctx context.Context, checkers checkersMap, tx *sbac.Transaction) (bool, error) {
	ctpairs, err := aggregate(checkers, tx.Traces)
	if err != nil {
		return false, err
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, v := range ctpairs {
		v := v
		t := v.Trace
		c := v.Checker
		g.Go(func() error {
			result := c.Check(t.InputObjects, t.InputReferences, t.Parameters,
				t.OutputObjects, t.Returns, t.Dependencies)

			if !result {
				return errors.New("check failed")
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return false, nil
	}
	return true, nil
}
