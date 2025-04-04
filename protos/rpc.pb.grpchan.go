// Code generated by protoc-gen-grpchan. DO NOT EDIT.
// source: rpc.proto

package protos

import "github.com/fullstorydev/grpchan"
import "golang.org/x/net/context"
import "google.golang.org/grpc"

func RegisterHandlerRPC(reg grpchan.ServiceRegistry, srv RPCServer) {
	reg.RegisterService(&RPC_ServiceDesc, srv)
}

type rPCChannelClient struct {
	ch grpchan.Channel
}

func NewRPCChannelClient(ch grpchan.Channel) RPCClient {
	return &rPCChannelClient{ch: ch}
}

func (c *rPCChannelClient) Connect(ctx context.Context, in *ConnectionRequest, opts ...grpc.CallOption) (*ConnectionResponse, error) {
	out := new(ConnectionResponse)
	err := c.ch.Invoke(ctx, "/anycable.RPC/Connect", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *rPCChannelClient) Command(ctx context.Context, in *CommandMessage, opts ...grpc.CallOption) (*CommandResponse, error) {
	out := new(CommandResponse)
	err := c.ch.Invoke(ctx, "/anycable.RPC/Command", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *rPCChannelClient) Disconnect(ctx context.Context, in *DisconnectRequest, opts ...grpc.CallOption) (*DisconnectResponse, error) {
	out := new(DisconnectResponse)
	err := c.ch.Invoke(ctx, "/anycable.RPC/Disconnect", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}
