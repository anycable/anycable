package rpc

import (
	"github.com/fullstorydev/grpchan"
	"github.com/fullstorydev/grpchan/inprocgrpc"

	pb "github.com/anycable/anycable-go/protos"
)

func NewInprocessServiceDialer(service pb.RPCServer, stateHandler ClientHelper) Dialer {
	handlers := grpchan.HandlerMap{}
	inproc := &inprocgrpc.Channel{}

	pb.RegisterHandlerRPC(handlers, service)
	handlers.ForEach(inproc.RegisterService)

	return func(c *Config) (pb.RPCClient, ClientHelper, error) {
		inprocClient := pb.NewRPCChannelClient(inproc)
		return inprocClient, stateHandler, nil
	}
}
