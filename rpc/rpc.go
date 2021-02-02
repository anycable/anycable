package rpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/protocol"
	"github.com/apex/log"

	pb "github.com/anycable/anycable-go/protos"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// ProtoVersions contains a comma-seprated list of compatible RPC protos versions
	// (we pass it as request meta to notify clients)
	ProtoVersions = "v0,v1"
	invokeTimeout = 3000

	retryExhaustedInterval   = 10
	retryUnavailableInterval = 100

	metricsRPCCalls    = "rpc_call_total"
	metricsRPCRetries  = "rpc_retries_total"
	metricsRPCFailures = "rpc_error_total"
	metricsRPCPending  = "rpc_pending_num"
)

// Controller implements node.Controller interface for gRPC
type Controller struct {
	config  *Config
	sem     chan (struct{})
	conn    *grpc.ClientConn
	client  pb.RPCClient
	metrics *metrics.Metrics
	log     *log.Entry
}

// NewController builds new Controller
func NewController(metrics *metrics.Metrics, config *Config) *Controller {
	metrics.RegisterCounter(metricsRPCCalls, "The total number of RPC calls")
	metrics.RegisterCounter(metricsRPCRetries, "The total number of RPC call retries")
	metrics.RegisterCounter(metricsRPCFailures, "The total number of failed RPC calls")
	metrics.RegisterGauge(metricsRPCPending, "The number of pending RPC calls")

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
		grpc.WithBalancerName("round_robin"), // nolint:staticcheck
	)

	c.initSemaphore(capacity)

	if err == nil {
		c.log.Infof("RPC controller initialized: %s (concurrency: %d, proto_versions: %s)", host, capacity, ProtoVersions)
	}

	c.conn = conn
	c.client = pb.NewRPCClient(conn)
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
	_, err := c.retry(func() (interface{}, error) {
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
func (c *Controller) Authenticate(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	c.metrics.Gauge(metricsRPCPending).Inc()
	<-c.sem
	defer func() { c.sem <- struct{}{} }()
	c.metrics.Gauge(metricsRPCPending).Dec()

	op := func() (interface{}, error) {
		return c.client.Connect(
			newContext(sid),
			protocol.NewConnectMessage(env),
		)
	}

	c.metrics.Counter(metricsRPCCalls).Inc()

	response, err := c.retry(op)

	if err != nil {
		c.metrics.Counter(metricsRPCFailures).Inc()

		return nil, err
	}

	if r, ok := response.(*pb.ConnectionResponse); ok {

		c.log.Debugf("Authenticate response: %v", r)

		reply, err := protocol.ParseConnectResponse(r)

		return reply, err
	}

	c.metrics.Counter(metricsRPCFailures).Inc()

	return nil, errors.New("Failed to deserialize connection response")
}

// Subscribe performs Command RPC call with "subscribe" command
func (c *Controller) Subscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	c.metrics.Gauge(metricsRPCPending).Inc()
	<-c.sem
	defer func() { c.sem <- struct{}{} }()
	c.metrics.Gauge(metricsRPCPending).Dec()

	op := func() (interface{}, error) {
		return c.client.Command(
			newContext(sid),
			protocol.NewCommandMessage(env, "subscribe", channel, id, ""),
		)
	}

	response, err := c.retry(op)

	return c.parseCommandResponse(response, err)
}

// Unsubscribe performs Command RPC call with "unsubscribe" command
func (c *Controller) Unsubscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	c.metrics.Gauge(metricsRPCPending).Inc()
	<-c.sem
	defer func() { c.sem <- struct{}{} }()
	c.metrics.Gauge(metricsRPCPending).Dec()

	op := func() (interface{}, error) {
		return c.client.Command(
			newContext(sid),
			protocol.NewCommandMessage(env, "unsubscribe", channel, id, ""),
		)
	}

	response, err := c.retry(op)

	return c.parseCommandResponse(response, err)
}

// Perform performs Command RPC call with "perform" command
func (c *Controller) Perform(sid string, env *common.SessionEnv, id string, channel string, data string) (*common.CommandResult, error) {
	c.metrics.Gauge(metricsRPCPending).Inc()
	<-c.sem
	defer func() { c.sem <- struct{}{} }()
	c.metrics.Gauge(metricsRPCPending).Dec()

	op := func() (interface{}, error) {
		return c.client.Command(
			newContext(sid),
			protocol.NewCommandMessage(env, "message", channel, id, data),
		)
	}

	response, err := c.retry(op)

	return c.parseCommandResponse(response, err)
}

// Disconnect performs disconnect RPC call
func (c *Controller) Disconnect(sid string, env *common.SessionEnv, id string, subscriptions []string) error {
	c.metrics.Gauge(metricsRPCPending).Inc()
	<-c.sem
	defer func() { c.sem <- struct{}{} }()
	c.metrics.Gauge(metricsRPCPending).Dec()

	op := func() (interface{}, error) {
		return c.client.Disconnect(
			newContext(sid),
			protocol.NewDisconnectMessage(env, id, subscriptions),
		)
	}

	c.metrics.Counter(metricsRPCCalls).Inc()

	response, err := c.retry(op)

	if err != nil {
		c.metrics.Counter(metricsRPCFailures).Inc()
		return err
	}

	if r, ok := response.(*pb.DisconnectResponse); ok {
		c.log.Debugf("Disconnect response: %v", r)

		err = protocol.ParseDisconnectResponse(r)

		if err != nil {
			c.metrics.Counter(metricsRPCFailures).Inc()
		}

		return err
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

		res, err := protocol.ParseCommandResponse(r)

		return res, err
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

func (c *Controller) retry(callback func() (interface{}, error)) (res interface{}, err error) {
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

		code := st.Code()

		if !(code == codes.ResourceExhausted || code == codes.Unavailable) {
			return nil, err
		}

		c.log.WithField("code", st.Code()).Debugf("RPC failure: %v", st.Message())

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

		c.metrics.Counter(metricsRPCRetries).Inc()

		time.Sleep(delay * time.Millisecond)

		attempt++
	}
}

func (c *Controller) initSemaphore(capacity int) {
	c.sem = make(chan struct{}, capacity)
	for i := 0; i < capacity; i++ {
		c.sem <- struct{}{}
	}
}

func newContext(sessionID string) context.Context {
	md := metadata.Pairs("sid", sessionID, "protov", ProtoVersions)
	return metadata.NewOutgoingContext(context.Background(), md)
}
