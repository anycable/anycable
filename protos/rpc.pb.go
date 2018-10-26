// Code generated by protoc-gen-go.
// source: rpc.proto
// DO NOT EDIT!

/*
Package anycable is a generated protocol buffer package.

It is generated from these files:
	rpc.proto

It has these top-level messages:
	ConnectionRequest
	ConnectionResponse
	CommandMessage
	CommandResponse
	DisconnectRequest
	DisconnectResponse
*/
package anycable

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type Status int32

const (
	Status_ERROR   Status = 0
	Status_SUCCESS Status = 1
	Status_FAILURE Status = 2
)

var Status_name = map[int32]string{
	0: "ERROR",
	1: "SUCCESS",
	2: "FAILURE",
}
var Status_value = map[string]int32{
	"ERROR":   0,
	"SUCCESS": 1,
	"FAILURE": 2,
}

func (x Status) String() string {
	return proto.EnumName(Status_name, int32(x))
}
func (Status) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type ConnectionRequest struct {
	Path    string            `protobuf:"bytes,1,opt,name=path" json:"path,omitempty"`
	Headers map[string]string `protobuf:"bytes,2,rep,name=headers" json:"headers,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
}

func (m *ConnectionRequest) Reset()                    { *m = ConnectionRequest{} }
func (m *ConnectionRequest) String() string            { return proto.CompactTextString(m) }
func (*ConnectionRequest) ProtoMessage()               {}
func (*ConnectionRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *ConnectionRequest) GetHeaders() map[string]string {
	if m != nil {
		return m.Headers
	}
	return nil
}

type ConnectionResponse struct {
	Status        Status   `protobuf:"varint,1,opt,name=status,enum=anycable.Status" json:"status,omitempty"`
	Identifiers   string   `protobuf:"bytes,2,opt,name=identifiers" json:"identifiers,omitempty"`
	Transmissions []string `protobuf:"bytes,3,rep,name=transmissions" json:"transmissions,omitempty"`
	ErrorMsg      string   `protobuf:"bytes,4,opt,name=error_msg,json=errorMsg" json:"error_msg,omitempty"`
}

func (m *ConnectionResponse) Reset()                    { *m = ConnectionResponse{} }
func (m *ConnectionResponse) String() string            { return proto.CompactTextString(m) }
func (*ConnectionResponse) ProtoMessage()               {}
func (*ConnectionResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

type CommandMessage struct {
	Command               string `protobuf:"bytes,1,opt,name=command" json:"command,omitempty"`
	Identifier            string `protobuf:"bytes,2,opt,name=identifier" json:"identifier,omitempty"`
	ConnectionIdentifiers string `protobuf:"bytes,3,opt,name=connection_identifiers,json=connectionIdentifiers" json:"connection_identifiers,omitempty"`
	Data                  string `protobuf:"bytes,4,opt,name=data" json:"data,omitempty"`
}

func (m *CommandMessage) Reset()                    { *m = CommandMessage{} }
func (m *CommandMessage) String() string            { return proto.CompactTextString(m) }
func (*CommandMessage) ProtoMessage()               {}
func (*CommandMessage) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

type CommandResponse struct {
	Status        Status   `protobuf:"varint,1,opt,name=status,enum=anycable.Status" json:"status,omitempty"`
	Disconnect    bool     `protobuf:"varint,2,opt,name=disconnect" json:"disconnect,omitempty"`
	StopStreams   bool     `protobuf:"varint,3,opt,name=stop_streams,json=stopStreams" json:"stop_streams,omitempty"`
	Streams       []string `protobuf:"bytes,4,rep,name=streams" json:"streams,omitempty"`
	Transmissions []string `protobuf:"bytes,5,rep,name=transmissions" json:"transmissions,omitempty"`
	ErrorMsg      string   `protobuf:"bytes,6,opt,name=error_msg,json=errorMsg" json:"error_msg,omitempty"`
}

func (m *CommandResponse) Reset()                    { *m = CommandResponse{} }
func (m *CommandResponse) String() string            { return proto.CompactTextString(m) }
func (*CommandResponse) ProtoMessage()               {}
func (*CommandResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

type DisconnectRequest struct {
	Identifiers   string            `protobuf:"bytes,1,opt,name=identifiers" json:"identifiers,omitempty"`
	Subscriptions []string          `protobuf:"bytes,2,rep,name=subscriptions" json:"subscriptions,omitempty"`
	Path          string            `protobuf:"bytes,3,opt,name=path" json:"path,omitempty"`
	Headers       map[string]string `protobuf:"bytes,4,rep,name=headers" json:"headers,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
}

func (m *DisconnectRequest) Reset()                    { *m = DisconnectRequest{} }
func (m *DisconnectRequest) String() string            { return proto.CompactTextString(m) }
func (*DisconnectRequest) ProtoMessage()               {}
func (*DisconnectRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *DisconnectRequest) GetHeaders() map[string]string {
	if m != nil {
		return m.Headers
	}
	return nil
}

type DisconnectResponse struct {
	Status   Status `protobuf:"varint,1,opt,name=status,enum=anycable.Status" json:"status,omitempty"`
	ErrorMsg string `protobuf:"bytes,2,opt,name=error_msg,json=errorMsg" json:"error_msg,omitempty"`
}

func (m *DisconnectResponse) Reset()                    { *m = DisconnectResponse{} }
func (m *DisconnectResponse) String() string            { return proto.CompactTextString(m) }
func (*DisconnectResponse) ProtoMessage()               {}
func (*DisconnectResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func init() {
	proto.RegisterType((*ConnectionRequest)(nil), "anycable.ConnectionRequest")
	proto.RegisterType((*ConnectionResponse)(nil), "anycable.ConnectionResponse")
	proto.RegisterType((*CommandMessage)(nil), "anycable.CommandMessage")
	proto.RegisterType((*CommandResponse)(nil), "anycable.CommandResponse")
	proto.RegisterType((*DisconnectRequest)(nil), "anycable.DisconnectRequest")
	proto.RegisterType((*DisconnectResponse)(nil), "anycable.DisconnectResponse")
	proto.RegisterEnum("anycable.Status", Status_name, Status_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion3

// Client API for RPC service

type RPCClient interface {
	Connect(ctx context.Context, in *ConnectionRequest, opts ...grpc.CallOption) (*ConnectionResponse, error)
	Command(ctx context.Context, in *CommandMessage, opts ...grpc.CallOption) (*CommandResponse, error)
	Disconnect(ctx context.Context, in *DisconnectRequest, opts ...grpc.CallOption) (*DisconnectResponse, error)
}

type rPCClient struct {
	cc *grpc.ClientConn
}

func NewRPCClient(cc *grpc.ClientConn) RPCClient {
	return &rPCClient{cc}
}

func (c *rPCClient) Connect(ctx context.Context, in *ConnectionRequest, opts ...grpc.CallOption) (*ConnectionResponse, error) {
	out := new(ConnectionResponse)
	err := grpc.Invoke(ctx, "/anycable.RPC/Connect", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *rPCClient) Command(ctx context.Context, in *CommandMessage, opts ...grpc.CallOption) (*CommandResponse, error) {
	out := new(CommandResponse)
	err := grpc.Invoke(ctx, "/anycable.RPC/Command", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *rPCClient) Disconnect(ctx context.Context, in *DisconnectRequest, opts ...grpc.CallOption) (*DisconnectResponse, error) {
	out := new(DisconnectResponse)
	err := grpc.Invoke(ctx, "/anycable.RPC/Disconnect", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for RPC service

type RPCServer interface {
	Connect(context.Context, *ConnectionRequest) (*ConnectionResponse, error)
	Command(context.Context, *CommandMessage) (*CommandResponse, error)
	Disconnect(context.Context, *DisconnectRequest) (*DisconnectResponse, error)
}

func RegisterRPCServer(s *grpc.Server, srv RPCServer) {
	s.RegisterService(&_RPC_serviceDesc, srv)
}

func _RPC_Connect_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConnectionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RPCServer).Connect(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/anycable.RPC/Connect",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RPCServer).Connect(ctx, req.(*ConnectionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _RPC_Command_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CommandMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RPCServer).Command(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/anycable.RPC/Command",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RPCServer).Command(ctx, req.(*CommandMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _RPC_Disconnect_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DisconnectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RPCServer).Disconnect(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/anycable.RPC/Disconnect",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RPCServer).Disconnect(ctx, req.(*DisconnectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _RPC_serviceDesc = grpc.ServiceDesc{
	ServiceName: "anycable.RPC",
	HandlerType: (*RPCServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Connect",
			Handler:    _RPC_Connect_Handler,
		},
		{
			MethodName: "Command",
			Handler:    _RPC_Command_Handler,
		},
		{
			MethodName: "Disconnect",
			Handler:    _RPC_Disconnect_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: fileDescriptor0,
}

func init() { proto.RegisterFile("rpc.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 536 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xb4, 0x54, 0xdd, 0x8a, 0x13, 0x4d,
	0x10, 0x4d, 0xcf, 0xe4, 0xb7, 0xb2, 0xdf, 0x7e, 0xd9, 0x42, 0x65, 0xcc, 0xca, 0x12, 0x07, 0x2f,
	0x82, 0x60, 0x2e, 0x22, 0x82, 0xec, 0x95, 0x1a, 0xb3, 0x18, 0x70, 0x51, 0x3a, 0xec, 0x95, 0x17,
	0xa1, 0x33, 0x69, 0xb3, 0x83, 0x3b, 0x3f, 0x76, 0x77, 0x84, 0x3c, 0x88, 0x4f, 0xe0, 0x85, 0xef,
	0xa4, 0x0f, 0xe1, 0x2b, 0xc8, 0x74, 0xcf, 0x24, 0x9d, 0x4c, 0x50, 0x14, 0xbc, 0xeb, 0x3a, 0x55,
	0xa9, 0x73, 0xea, 0x54, 0x6a, 0xa0, 0x25, 0xd2, 0x60, 0x90, 0x8a, 0x44, 0x25, 0xd8, 0x64, 0xf1,
	0x3a, 0x60, 0xf3, 0x1b, 0xee, 0x7f, 0x25, 0x70, 0x32, 0x4a, 0xe2, 0x98, 0x07, 0x2a, 0x4c, 0x62,
	0xca, 0x3f, 0xae, 0xb8, 0x54, 0x88, 0x50, 0x4d, 0x99, 0xba, 0xf6, 0x48, 0x8f, 0xf4, 0x5b, 0x54,
	0xbf, 0xf1, 0x05, 0x34, 0xae, 0x39, 0x5b, 0x70, 0x21, 0x3d, 0xa7, 0xe7, 0xf6, 0xdb, 0xc3, 0xfe,
	0xa0, 0xe8, 0x32, 0x28, 0x75, 0x18, 0xbc, 0x32, 0xa5, 0xe3, 0x58, 0x89, 0x35, 0x2d, 0x7e, 0xd8,
	0x3d, 0x87, 0x23, 0x3b, 0x81, 0x1d, 0x70, 0x3f, 0xf0, 0x75, 0x4e, 0x93, 0x3d, 0xf1, 0x16, 0xd4,
	0x3e, 0xb1, 0x9b, 0x15, 0xf7, 0x1c, 0x8d, 0x99, 0xe0, 0xdc, 0x79, 0x4a, 0xfc, 0x2f, 0x04, 0xd0,
	0xe6, 0x91, 0x69, 0x12, 0x4b, 0x8e, 0x7d, 0xa8, 0x4b, 0xc5, 0xd4, 0x4a, 0xea, 0x2e, 0xc7, 0xc3,
	0xce, 0x56, 0xd5, 0x54, 0xe3, 0x34, 0xcf, 0x63, 0x0f, 0xda, 0xe1, 0x82, 0xc7, 0x2a, 0x7c, 0x1f,
	0x9a, 0x21, 0x32, 0x02, 0x1b, 0xc2, 0x07, 0xf0, 0x9f, 0x12, 0x2c, 0x96, 0x51, 0x28, 0x65, 0x98,
	0xc4, 0xd2, 0x73, 0x7b, 0x6e, 0xbf, 0x45, 0x77, 0x41, 0x3c, 0x85, 0x16, 0x17, 0x22, 0x11, 0xb3,
	0x48, 0x2e, 0xbd, 0xaa, 0xee, 0xd2, 0xd4, 0xc0, 0xa5, 0x5c, 0xfa, 0x9f, 0x09, 0x1c, 0x8f, 0x92,
	0x28, 0x62, 0xf1, 0xe2, 0x92, 0x4b, 0xc9, 0x96, 0x1c, 0x3d, 0x68, 0x04, 0x06, 0xc9, 0x07, 0x2d,
	0x42, 0x3c, 0x03, 0xd8, 0xd2, 0xe7, 0x82, 0x2c, 0x04, 0x9f, 0xc0, 0x9d, 0x60, 0x33, 0xf1, 0xcc,
	0x16, 0xef, 0xea, 0xda, 0xdb, 0xdb, 0xec, 0xc4, 0x1a, 0x03, 0xa1, 0xba, 0x60, 0x8a, 0xe5, 0xda,
	0xf4, 0xdb, 0xff, 0x4e, 0xe0, 0xff, 0x5c, 0xd7, 0x5f, 0x58, 0x77, 0x06, 0xb0, 0x08, 0x65, 0xce,
	0xa6, 0x85, 0x36, 0xa9, 0x85, 0xe0, 0x7d, 0x38, 0x92, 0x2a, 0x49, 0x67, 0x52, 0x09, 0xce, 0x22,
	0x23, 0xaf, 0x49, 0xdb, 0x19, 0x36, 0x35, 0x50, 0xe6, 0x42, 0x91, 0xad, 0x6a, 0x57, 0x8b, 0xb0,
	0xec, 0x7a, 0xed, 0xb7, 0xae, 0xd7, 0xf7, 0x5c, 0xff, 0x41, 0xe0, 0xe4, 0xe5, 0x46, 0x4e, 0xf1,
	0x2f, 0xde, 0x5b, 0x38, 0x39, 0xb8, 0x70, 0xb9, 0x9a, 0xcb, 0x40, 0x84, 0xa9, 0xd2, 0xd4, 0x8e,
	0xa1, 0xde, 0x01, 0x37, 0xd7, 0xe0, 0x1e, 0xbe, 0x86, 0xea, 0xfe, 0x35, 0x94, 0x94, 0xfc, 0x83,
	0x6b, 0x78, 0x07, 0x68, 0xd3, 0xfc, 0xf1, 0x46, 0x77, 0xec, 0x74, 0x76, 0xed, 0x7c, 0xf8, 0x08,
	0xea, 0xa6, 0x1c, 0x5b, 0x50, 0x1b, 0x53, 0xfa, 0x86, 0x76, 0x2a, 0xd8, 0x86, 0xc6, 0xf4, 0x6a,
	0x34, 0x1a, 0x4f, 0xa7, 0x1d, 0x92, 0x05, 0x17, 0xcf, 0x27, 0xaf, 0xaf, 0xe8, 0xb8, 0xe3, 0x0c,
	0xbf, 0x11, 0x70, 0xe9, 0xdb, 0x11, 0x5e, 0x40, 0x23, 0x3f, 0x50, 0x3c, 0xfd, 0xc5, 0xb7, 0xa1,
	0x7b, 0xef, 0x70, 0xd2, 0xcc, 0xe0, 0x57, 0xf0, 0x59, 0xd6, 0xc7, 0x5c, 0x88, 0x67, 0x97, 0xda,
	0x57, 0xd5, 0xbd, 0x5b, 0xca, 0x58, 0x1d, 0x26, 0x00, 0x5b, 0x77, 0x6c, 0x31, 0xa5, 0xd5, 0xd8,
	0x62, 0xca, 0x86, 0xfa, 0x95, 0x79, 0x5d, 0x7f, 0x31, 0x1f, 0xff, 0x0c, 0x00, 0x00, 0xff, 0xff,
	0x69, 0xde, 0x1c, 0x58, 0x3e, 0x05, 0x00, 0x00,
}