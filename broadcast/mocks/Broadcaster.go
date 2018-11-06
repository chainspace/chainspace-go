// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import broadcast "chainspace.io/prototype/broadcast"
import mock "github.com/stretchr/testify/mock"

// Broadcaster is an autogenerated mock type for the Broadcaster type
type Broadcaster struct {
	mock.Mock
}

// AddTransaction provides a mock function with given fields: txdata, fee
func (_m *Broadcaster) AddTransaction(txdata []byte, fee uint64) error {
	ret := _m.Called(txdata, fee)

	var r0 error
	if rf, ok := ret.Get(0).(func([]byte, uint64) error); ok {
		r0 = rf(txdata, fee)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Register provides a mock function with given fields: cb
func (_m *Broadcaster) Register(cb broadcast.Callback) {
	_m.Called(cb)
}
