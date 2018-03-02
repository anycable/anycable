package rpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/node"
	"github.com/apex/log"

	grpcpool "github.com/anycable/anycable-go/pool"
	pb "github.com/anycable/anycable-go/protos"
	"google.golang.org/grpc"
)

const (
	retryInterval = 500
	invokeTimeout = 3000

	initialCapacity = 5
	maxCapacity     = 50
)

// Controller implements node.Controller interface for gRPC
type Controller struct {
	host string
	pool grpcpool.Pool
	log  *log.Entry
}

// NewController builds new Controller from config
func NewController(config *config.Config) *Controller {
	return &Controller{log: log.WithField("context", "rpc"), host: config.RPCHost}
}

// Start initializes RPC connection pool
func (c *Controller) Start() error {
	host := c.host

	factory := func() (*grpc.ClientConn, error) {
		return grpc.Dial(host, grpc.WithInsecure())
	}

	pool, err := grpcpool.NewChannelPool(initialCapacity, maxCapacity, factory)

	if err == nil {
		c.log.Infof("RPC pool initialized: %s", host)
	}

	c.pool = pool
	return err
}

// Shutdown closes connections
func (c *Controller) Shutdown() error {
	c.pool.Close()

	busy := c.pool.Busy()

	if busy > 0 {
		c.log.Infof("Waiting for active RPC calls to finish: %d", busy)
	}

	// Wait for active connections
	_, err := retry(func() (interface{}, error) {
		busy := c.pool.Busy()

		if busy > 0 {
			return false, fmt.Errorf("There are %d active RPC connections left", busy)
		}

		return true, nil
	})

	return err
}

// Authenticate performs Connect RPC call
func (c *Controller) Authenticate(path string, headers *map[string]string) (string, []string, error) {
	conn, err := c.getConn()

	if err != nil {
		return "", nil, err
	}

	defer conn.Close()

	client := pb.NewRPCClient(conn.Conn)

	op := func() (interface{}, error) {
		return client.Connect(context.Background(), &pb.ConnectionRequest{Path: path, Headers: *headers})
	}

	response, err := retry(op)

	if err != nil {
		return "", nil, err
	}

	if r, ok := response.(*pb.ConnectionResponse); ok {

		c.log.Debugf("Authenticate response: %v", r)

		if r.Status.String() == "SUCCESS" {
			return r.Identifiers, r.Transmissions, nil
		}

		return "", nil, fmt.Errorf("Application error: %s", r.ErrorMsg)
	}

	return "", nil, errors.New("Failed to deserialize connection response")
}

// Subscribe performs Command RPC call with "subscribe" command
func (c *Controller) Subscribe(sid string, id string, channel string) (*node.CommandResult, error) {
	conn, err := c.getConn()

	if err != nil {
		return nil, err
	}

	defer conn.Close()

	client := pb.NewRPCClient(conn.Conn)

	op := func() (interface{}, error) {
		return client.Command(context.Background(), &pb.CommandMessage{Command: "subscribe", Identifier: channel, ConnectionIdentifiers: id})
	}

	response, err := retry(op)

	return c.parseCommandResponse(response, err)
}

// Unsubscribe performs Command RPC call with "unsubscribe" command
func (c *Controller) Unsubscribe(sid string, id string, channel string) (*node.CommandResult, error) {
	conn, err := c.getConn()

	if err != nil {
		return nil, err
	}

	defer conn.Close()

	client := pb.NewRPCClient(conn.Conn)

	op := func() (interface{}, error) {
		return client.Command(context.Background(), &pb.CommandMessage{Command: "unsubscribe", Identifier: channel, ConnectionIdentifiers: id})
	}

	response, err := retry(op)

	return c.parseCommandResponse(response, err)
}

// Perform performs Command RPC call with "perform" command
func (c *Controller) Perform(sid string, id string, channel string, data string) (*node.CommandResult, error) {
	conn, err := c.getConn()

	if err != nil {
		return nil, err
	}

	defer conn.Close()

	client := pb.NewRPCClient(conn.Conn)

	op := func() (interface{}, error) {
		return client.Command(context.Background(), &pb.CommandMessage{Command: "message", Identifier: channel, ConnectionIdentifiers: id, Data: data})
	}

	response, err := retry(op)

	return c.parseCommandResponse(response, err)
}

// Disconnect performs disconnect RPC call
func (c *Controller) Disconnect(sid string, id string, subscriptions []string, path string, headers *map[string]string) error {
	conn, err := c.getConn()

	if err != nil {
		return err
	}

	defer conn.Close()

	client := pb.NewRPCClient(conn.Conn)

	op := func() (interface{}, error) {
		return client.Disconnect(context.Background(), &pb.DisconnectRequest{Identifiers: id, Subscriptions: subscriptions, Path: path, Headers: *headers})
	}

	response, err := retry(op)

	if err != nil {
		return err
	}

	if r, ok := response.(*pb.DisconnectResponse); ok {
		c.log.Debugf("Disconnect response: %v", r)

		if r.Status.String() == "SUCCESS" {
			return nil
		}

		return fmt.Errorf("Application error: %s", r.ErrorMsg)
	}

	return errors.New("Failed to deserialize disconnect response")
}

func (c *Controller) parseCommandResponse(response interface{}, err error) (*node.CommandResult, error) {
	if err != nil {
		return nil, err
	}

	if r, ok := response.(*pb.CommandResponse); ok {
		c.log.Debugf("Command response: %v", r)

		res := &node.CommandResult{
			Disconnect:     r.Disconnect,
			StopAllStreams: r.StopStreams,
			Streams:        r.Streams,
			Transmissions:  r.Transmissions,
		}

		if r.Status.String() == "SUCCESS" {
			return res, nil
		}

		return res, fmt.Errorf("Application error: %s", r.ErrorMsg)
	}

	return nil, errors.New("Failed to deserialize command response")
}

func (c *Controller) getConn() (*grpcpool.Conn, error) {
	conn, err := c.pool.Get()

	if err != nil {
		return nil, err
	}

	return &conn, nil
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
