package transactor // import "chainspace.io/prototype/transactor"

import (
	"errors"
	"fmt"

	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
)

// Checker define the interface for a contract procedure check
type Checker interface {
	ContractID() string
	Name() string
	Check(inputs, refInputs, parameters, outputs, returns [][]byte, labels [][]string, dependencies []*Trace) bool
}

// a pair of a trace and it's associated checker to be store in a slice
type checkerTracePair struct {
	Checker Checker
	Trace   *Trace
}

// CheckersMap map contractID to map check procedure name to checker
type CheckersMap map[string]map[string]Checker

func runCheckers(ctx context.Context, checkers CheckersMap, tx *Transaction) (bool, error) {
	ctpairs, err := aggregateCheckers(checkers, tx.Traces)
	if err != nil {
		return false, err
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, v := range ctpairs {
		v := v
		t := v.Trace
		c := v.Checker
		g.Go(func() error {
			result := c.Check(t.InputObjectsKeys, t.InputReferencesKeys, t.Parameters, t.OutputObjects, t.Returns, StringsSlice(t.Labels).AsSlice(), t.Dependencies)
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

// aggregateCheckers first ensure that all the contracts and procedures used in the transaction
// exists then map each transaction to the associated contract.
func aggregateCheckers(checkers CheckersMap, traces []*Trace) ([]checkerTracePair, error) {
	var ok bool
	var m map[string]Checker
	var checker Checker
	ctpairs := []checkerTracePair{}
	for _, t := range traces {
		m, ok = checkers[t.ContractID]
		if !ok {
			return nil, fmt.Errorf("transactor: unknown contract with ID: %v", t.ContractID)
		}
		checker, ok = m[t.Procedure]
		if !ok {
			return nil, fmt.Errorf("transactor: unknown procedure %v for contract with ID: %v", t.Procedure, t.ContractID)
		}
		newpairs, err := aggregateCheckers(checkers, t.Dependencies)
		if err != nil {
			return nil, err
		}
		newpair := checkerTracePair{Checker: checker, Trace: t}
		ctpairs = append(ctpairs, newpair)
		ctpairs = append(ctpairs, newpairs...)
	}
	return ctpairs, nil
}
