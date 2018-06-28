package transactor

// Checker define the interface for a contract procedure check
type Checker interface {
	ContractID() string
	Name() string
	Check(inputs, refInputs, parameters, outputs, returns [][]byte, dependencies []*Trace) bool
}
