package main

import (
	"net/http"
	"time"

	"github.com/anycable/anycable-go/pool"
	pb "github.com/anycable/anycable-go/protos"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	retryInterval = 500
	invokeTimeout = 3000
)

type Remote struct {
	pool pool.Pool
}

var rpc = Remote{}

func (rpc *Remote) Init(host string) {
	factory := func() (*grpc.ClientConn, error) {
		return grpc.Dial(host, grpc.WithInsecure())
	}

	p, err := pool.NewChannelPool(5, 50, factory)

	if err != nil {
		log.Criticalf("failed to create pool: %v", err)
	}

	rpc.pool = p
}

func (rpc *Remote) GetConn() pool.PoolConn {
	pc, err := rpc.pool.Get()

	if err != nil {
		log.Criticalf("failed to retrieve connection: %v", err)
	}

	return pc
}

func (rpc *Remote) Close() {
	rpc.pool.Close()
}

func (rpc *Remote) VerifyConnection(r *http.Request) *pb.ConnectionResponse {
	conn := rpc.GetConn()
	defer conn.Close()
	client := pb.NewRPCClient(conn.Conn)

	op := func() (interface{}, error) {
		return client.Connect(context.Background(), &pb.ConnectionRequest{Path: r.URL.String(), Headers: GetHeaders(r)})
	}

	response, err := retry(op)

	if err != nil {
		log.Errorf("RPC Error: %v", err)
		return nil
	}

	if r, ok := response.(*pb.ConnectionResponse); ok {
		return r
	} else {
		return nil
	}
}

func (rpc *Remote) Subscribe(connId string, channelId string) *pb.CommandResponse {
	conn := rpc.GetConn()
	defer conn.Close()
	client := pb.NewRPCClient(conn.Conn)

	op := func() (interface{}, error) {
		return client.Command(context.Background(), &pb.CommandMessage{Command: "subscribe", Identifier: channelId, ConnectionIdentifiers: connId})
	}

	response, err := retry(op)

	return ParseCommandResponse(response, err)
}

func (rpc *Remote) Unsubscribe(connId string, channelId string) *pb.CommandResponse {
	conn := rpc.GetConn()
	defer conn.Close()
	client := pb.NewRPCClient(conn.Conn)

	op := func() (interface{}, error) {
		return client.Command(context.Background(), &pb.CommandMessage{Command: "unsubscribe", Identifier: channelId, ConnectionIdentifiers: connId})
	}

	response, err := retry(op)

	return ParseCommandResponse(response, err)
}

func (rpc *Remote) Perform(connId string, channelId string, data string) *pb.CommandResponse {
	conn := rpc.GetConn()
	defer conn.Close()
	client := pb.NewRPCClient(conn.Conn)

	op := func() (interface{}, error) {
		return client.Command(context.Background(), &pb.CommandMessage{Command: "message", Identifier: channelId, ConnectionIdentifiers: connId, Data: data})
	}

	response, err := retry(op)

	return ParseCommandResponse(response, err)
}

func (rpc *Remote) Disconnect(connId string, subscriptions []string) *pb.DisconnectResponse {
	conn := rpc.GetConn()
	defer conn.Close()
	client := pb.NewRPCClient(conn.Conn)

	op := func() (interface{}, error) {
		return client.Disconnect(context.Background(), &pb.DisconnectRequest{Identifiers: connId, Subscriptions: subscriptions})
	}

	response, err := retry(op)

	if err != nil {
		log.Errorf("RPC Error: %v", err)
		return nil
	}

	if r, ok := response.(*pb.DisconnectResponse); ok {
		return r
	} else {
		return nil
	}
}

func retry(callback func() (interface{}, error)) (res interface{}, err error) {
	attempts := invokeTimeout / retryInterval

	for i := 0; ; i++ {
		res, err = callback()

		if err == nil {
			return res, nil
		}

		if i >= (attempts - 1) {
			return nil, err
		}

		time.Sleep(retryInterval * time.Millisecond)
	}
}

func ParseCommandResponse(response interface{}, err error) *pb.CommandResponse {
	if err != nil {
		log.Errorf("RPC Error: %v", err)
		return nil
	}

	if r, ok := response.(*pb.CommandResponse); ok {
		return r
	} else {
		return nil
	}
}

func GetHeaders(r *http.Request) map[string]string {
	res := make(map[string]string)
	res["Cookie"] = r.Header.Get("Cookie")
	return res
}
