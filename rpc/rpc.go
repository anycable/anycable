package rpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/apex/log"

	pb "github.com/anycable/anycable-go/protos"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	invokeTimeout = 3000

	retryExhaustedInterval   = 10
	retryUnavailableInterval = 100

	metricsRPCCalls    = "rpc_call_total"
	metricsRPCFailures = "rpc_error_total"
)

// Controller implements node.Controller interface for gRPC
type Controller struct {
	config  *Config
	sem     chan (struct{})
	conn    *grpc.ClientConn
	metrics *metrics.Metrics
	log     *log.Entry
}

// NewController builds new Controller
func NewController(metrics *metrics.Metrics, config *Config) *Controller {
	metrics.RegisterCounter(metricsRPCCalls, "The total number of RPC calls")
	metrics.RegisterCounter(metricsRPCFailures, "The total number of failed RPC calls")

	return &Controller{log: log.WithField("context", "rpc"), metrics: metrics, config: config}
}

// Start initializes RPC connection pool
func (c *Controller) Start() error {
	host := c.config.Host
	capacity := c.config.Concurrency

	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		PermitWithoutStream: true,             // send pings even without active streams
	}

	conn, err := grpc.Dial(
		host,
		grpc.WithInsecure(),
		grpc.WithKeepaliveParams(kacp),
		grpc.WithBalancerName("round_robin"),
	)

	c.sem = make(chan struct{}, capacity)
	for i := 0; i < capacity; i++ {
		c.sem <- struct{}{}
	}

	if err == nil {
		c.log.Infof("RPC controller initialized: %s (concurrency %d)", host, capacity)
	}

	c.conn = conn
	return err
}

// Shutdown closes connections
func (c *Controller) Shutdown() error {
	if c.conn == nil {
		return nil
	}

	defer c.conn.Close()

	busy := c.busy()

	if busy > 0 {
		c.log.Infof("Waiting for active RPC calls to finish: %d", busy)
	}

	// Wait for active connections
	_, err := retry(func() (interface{}, error) {
		busy := c.busy()

		if busy > 0 {
			return false, fmt.Errorf("There are %d active RPC connections left", busy)
		}

		c.log.Info("All active RPC calls finished")
		return true, nil
	})

	return err
}

// Authenticate performs Connect RPC call
func (c *Controller) Authenticate(sid string, path string, headers *map[string]string) (string, []string, error) {
	<-c.sem
	defer func() { c.sem <- struct{}{} }()

	client := pb.NewRPCClient(c.conn)

	op := func() (interface{}, error) {
		return client.Connect(newContext(sid), &pb.ConnectionRequest{Path: path, Headers: *headers})
	}

	c.metrics.Counter(metricsRPCCalls).Inc()

	response, err := retry(op)

	if err != nil {
		c.metrics.Counter(metricsRPCFailures).Inc()

		return "", nil, err
	}

	if r, ok := response.(*pb.ConnectionResponse); ok {

		c.log.Debugf("Authenticate response: %v", r)

		if r.Status.String() == "SUCCESS" {
			return r.Identifiers, r.Transmissions, nil
		}

		return "", r.Transmissions, fmt.Errorf("Application error: %s", r.ErrorMsg)
	}

	c.metrics.Counter(metricsRPCFailures).Inc()

	return "", nil, errors.New("Failed to deserialize connection response")
}

// Subscribe performs Command RPC call with "subscribe" command
func (c *Controller) Subscribe(sid string, id string, channel string) (*common.CommandResult, error) {
	<-c.sem
	defer func() { c.sem <- struct{}{} }()

	client := pb.NewRPCClient(c.conn)

	op := func() (interface{}, error) {
		return client.Command(newContext(sid), &pb.CommandMessage{Command: "subscribe", Identifier: channel, ConnectionIdentifiers: id})
	}

	response, err := retry(op)

	return c.parseCommandResponse(response, err)
}

// Unsubscribe performs Command RPC call with "unsubscribe" command
func (c *Controller) Unsubscribe(sid string, id string, channel string) (*common.CommandResult, error) {
	<-c.sem
	defer func() { c.sem <- struct{}{} }()

	client := pb.NewRPCClient(c.conn)

	op := func() (interface{}, error) {
		return client.Command(newContext(sid), &pb.CommandMessage{Command: "unsubscribe", Identifier: channel, ConnectionIdentifiers: id})
	}

	response, err := retry(op)

	return c.parseCommandResponse(response, err)
}

// Perform performs Command RPC call with "perform" command
func (c *Controller) Perform(sid string, id string, channel string, data string) (*common.CommandResult, error) {
	<-c.sem
	defer func() { c.sem <- struct{}{} }()

	client := pb.NewRPCClient(c.conn)

	op := func() (interface{}, error) {
		return client.Command(newContext(sid), &pb.CommandMessage{Command: "message", Identifier: channel, ConnectionIdentifiers: id, Data: data})
	}

	response, err := retry(op)

	return c.parseCommandResponse(response, err)
}

// Disconnect performs disconnect RPC call
func (c *Controller) Disconnect(sid string, id string, subscriptions []string, path string, headers *map[string]string) error {
	<-c.sem
	defer func() { c.sem <- struct{}{} }()

	client := pb.NewRPCClient(c.conn)

	op := func() (interface{}, error) {
		return client.Disconnect(newContext(sid), &pb.DisconnectRequest{Identifiers: id, Subscriptions: subscriptions, Path: path, Headers: *headers})
	}

	c.metrics.Counter(metricsRPCCalls).Inc()

	response, err := retry(op)

	if err != nil {
		c.metrics.Counter(metricsRPCFailures).Inc()
		return err
	}

	if r, ok := response.(*pb.DisconnectResponse); ok {
		c.log.Debugf("Disconnect response: %v", r)

		if r.Status.String() == "SUCCESS" {
			return nil
		}

		c.metrics.Counter(metricsRPCFailures).Inc()

		return fmt.Errorf("Application error: %s", r.ErrorMsg)
	}

	return errors.New("Failed to deserialize disconnect response")
}

func (c *Controller) parseCommandResponse(response interface{}, err error) (*common.CommandResult, error) {
	c.metrics.Counter(metricsRPCCalls).Inc()

	if err != nil {
		c.metrics.Counter(metricsRPCFailures).Inc()

		return nil, err
	}

	if r, ok := response.(*pb.CommandResponse); ok {
		c.log.Debugf("Command response: %v", r)

		res := &common.CommandResult{
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

	c.metrics.Counter(metricsRPCFailures).Inc()

	return nil, errors.New("Failed to deserialize command response")
}

func (c *Controller) busy() int {
	// The number of in-flight request is the
	// the number of initial capacity "tickets" (concurrency)
	// minus the size of the semaphore channel
	return c.config.Concurrency - len(c.sem)
}

func retry(callback func() (interface{}, error)) (res interface{}, err error) {
	retryAge := 0
	attempt := 0
	wasExhausted := false

	for {
		res, err = callback()

		if err == nil {
			return res, nil
		}

		if retryAge > invokeTimeout {
			return nil, err
		}

		st, ok := status.FromError(err)
		if !ok {
			return nil, err
		}

		log.Debugf("RPC failure %v %v", st.Message(), st.Code())

		interval := retryUnavailableInterval

		if st.Code() == codes.ResourceExhausted {
			interval = retryExhaustedInterval
			if !wasExhausted {
				attempt = 0
				wasExhausted = true
			}
		} else if wasExhausted {
			wasExhausted = false
			attempt = 0
		}

		delayMS := int(math.Pow(2, float64(attempt))) * interval
		delay := time.Duration(delayMS)

		retryAge += delayMS

		time.Sleep(delay * time.Millisecond)

		attempt++
	}
}

func newContext(sessionID string) context.Context {
	md := metadata.Pairs("sid", sessionID)
	return metadata.NewOutgoingContext(context.Background(), md)
}
