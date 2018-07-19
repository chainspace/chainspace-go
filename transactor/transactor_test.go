package transactor

import (
	"errors"
	"testing"

	"golang.org/x/net/context"
)

func TestAggregateCheckers(t *testing.T) {
	testCases := []struct {
		ExpectedError error
		Checkers      CheckersMap
		Traces        []*Trace
		PairsCount    uint
	}{
		{
			ExpectedError: errors.New("transactor: unknown contract with ID: 200"),
			Checkers:      CheckersMap{},
			Traces: []*Trace{
				{
					ContractID: "200",
				},
			},
		},
		{
			ExpectedError: errors.New("transactor: unknown procedure 42 for contract with ID: contract_dummy"),
			Checkers: CheckersMap{
				"contract_dummy": map[string]Checker{
					"dummy_check_ok": &DummyCheckerOK{},
				},
			},
			Traces: []*Trace{
				{
					ContractID: "contract_dummy",
					Procedure:  "42",
				},
			},
		},
		{
			PairsCount:    2,
			ExpectedError: nil,
			Checkers: CheckersMap{
				"contract_dummy": map[string]Checker{
					"dummy_check_ok": &DummyCheckerOK{},
					"dummy_check_ko": &DummyCheckerKO{},
				},
			},
			Traces: []*Trace{
				{
					ContractID: "contract_dummy",
					Procedure:  "dummy_check_ok",
					Dependencies: []*Trace{
						{
							ContractID: "contract_dummy",
							Procedure:  "dummy_check_ko",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		pairs, err := aggregateCheckers(tc.Checkers, tc.Traces)
		if (err != nil && tc.ExpectedError != nil && err.Error() != tc.ExpectedError.Error()) || (err != nil && tc.ExpectedError == nil) || (err == nil && tc.ExpectedError != nil) {
			t.Errorf("expected error to be '%v' got '%v'", tc.ExpectedError, err)
		}
		if err == nil && tc.ExpectedError == nil && tc.PairsCount != uint(len(pairs)) {
			t.Errorf("expected %v pairs go %v", tc.PairsCount, len(pairs))
		}
	}
}

func TestRunCheckers(t *testing.T) {
	testCases := []struct {
		ExpectedError  error
		ExpectedResult bool
		Checkers       CheckersMap
		Transaction    Transaction
	}{
		{
			ExpectedResult: false,
			ExpectedError:  errors.New("transactor: unknown contract with ID: 200"),
			Checkers:       CheckersMap{},
			Transaction: Transaction{
				Traces: []*Trace{
					{
						ContractID: "200",
					},
				},
			},
		},
		{
			ExpectedResult: false,
			ExpectedError:  nil,
			Checkers: CheckersMap{
				"contract_dummy": map[string]Checker{
					"dummy_check_ok": &DummyCheckerOK{},
					"dummy_check_ko": &DummyCheckerKO{},
				},
			},
			Transaction: Transaction{
				Traces: []*Trace{
					{
						ContractID: "contract_dummy",
						Procedure:  "dummy_check_ok",
						Dependencies: []*Trace{
							{
								ContractID: "contract_dummy",
								Procedure:  "dummy_check_ko",
							},
						},
					},
				},
			},
		},
		{
			ExpectedResult: true,
			ExpectedError:  nil,
			Checkers: CheckersMap{
				"contract_dummy": map[string]Checker{
					"dummy_check_ok": &DummyCheckerOK{},
					"dummy_check_ko": &DummyCheckerKO{},
				},
			},
			Transaction: Transaction{
				Traces: []*Trace{
					{
						ContractID: "contract_dummy",
						Procedure:  "dummy_check_ok",
						Dependencies: []*Trace{
							{
								ContractID: "contract_dummy",
								Procedure:  "dummy_check_ok",
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		ok, err := runCheckers(context.TODO(), tc.Checkers, &tc.Transaction)
		if (err != nil && tc.ExpectedError != nil && err.Error() != tc.ExpectedError.Error()) || (err != nil && tc.ExpectedError == nil) || (err == nil && tc.ExpectedError != nil) {
			t.Errorf("expected error to be '%v' got '%v'", tc.ExpectedError, err)
		}
		if ok != tc.ExpectedResult {
			t.Errorf("expected result to be %v got %v", tc.ExpectedResult, ok)
		}
	}
}
