package transactor

type DummyCheckerOK struct{}

func (dc *DummyCheckerOK) Name() string { return "dummy_check_ok" }

func (dc *DummyCheckerOK) ContractID() string { return "contract_dummy" }

func (dc *DummyCheckerOK) Check(
	inputs, refInputs, parameters, outputs, returns [][]byte, dependencies []*Trace) bool {
	return true
}

type DummyCheckerKO struct{}

func (dc *DummyCheckerKO) Name() string { return "dummy_check_ko" }

func (dc *DummyCheckerKO) ContractID() string { return "contract_dummy" }

func (dc *DummyCheckerKO) Check(
	inputs, refInputs, parameters, outputs, returns [][]byte, dependencies []*Trace) bool {
	return false
}
