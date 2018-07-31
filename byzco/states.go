package byzco

var actions = map[status]handler{
	initialState: handleInitialState,
}

var transitions = map[transition]handler{}

func handleInitialState(i *instance) (status, error) {
	return committed, nil
}
