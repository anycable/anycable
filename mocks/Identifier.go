// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import common "github.com/anycable/anycable-go/common"

import mock "github.com/stretchr/testify/mock"

// Identifier is an autogenerated mock type for the Identifier type
type Identifier struct {
	mock.Mock
}

// Identify provides a mock function with given fields: sid, env
func (_m *Identifier) Identify(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	ret := _m.Called(sid, env)

	var r0 *common.ConnectResult
	if rf, ok := ret.Get(0).(func(string, *common.SessionEnv) *common.ConnectResult); ok {
		r0 = rf(sid, env)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*common.ConnectResult)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, *common.SessionEnv) error); ok {
		r1 = rf(sid, env)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}