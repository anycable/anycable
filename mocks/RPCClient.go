// Code generated by mockery v2.50.0. DO NOT EDIT.

package mocks

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"

	protos "github.com/anycable/anycable-go/protos"
)

// RPCClient is an autogenerated mock type for the RPCClient type
type RPCClient struct {
	mock.Mock
}

// Command provides a mock function with given fields: ctx, in, opts
func (_m *RPCClient) Command(ctx context.Context, in *protos.CommandMessage, opts ...grpc.CallOption) (*protos.CommandResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Command")
	}

	var r0 *protos.CommandResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *protos.CommandMessage, ...grpc.CallOption) (*protos.CommandResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *protos.CommandMessage, ...grpc.CallOption) *protos.CommandResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*protos.CommandResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *protos.CommandMessage, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Connect provides a mock function with given fields: ctx, in, opts
func (_m *RPCClient) Connect(ctx context.Context, in *protos.ConnectionRequest, opts ...grpc.CallOption) (*protos.ConnectionResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Connect")
	}

	var r0 *protos.ConnectionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *protos.ConnectionRequest, ...grpc.CallOption) (*protos.ConnectionResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *protos.ConnectionRequest, ...grpc.CallOption) *protos.ConnectionResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*protos.ConnectionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *protos.ConnectionRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Disconnect provides a mock function with given fields: ctx, in, opts
func (_m *RPCClient) Disconnect(ctx context.Context, in *protos.DisconnectRequest, opts ...grpc.CallOption) (*protos.DisconnectResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Disconnect")
	}

	var r0 *protos.DisconnectResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *protos.DisconnectRequest, ...grpc.CallOption) (*protos.DisconnectResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *protos.DisconnectRequest, ...grpc.CallOption) *protos.DisconnectResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*protos.DisconnectResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *protos.DisconnectRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewRPCClient creates a new instance of RPCClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewRPCClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *RPCClient {
	mock := &RPCClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
