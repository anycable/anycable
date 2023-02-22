package rpc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/protocol"
	"github.com/apex/log"

	pb "github.com/anycable/anycable-go/protos"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

const (
	// ProtoVersions contains a comma-seprated list of compatible RPC protos versions
	// (we pass it as request meta to notify clients)
	ProtoVersions = "v1"
	invokeTimeout = 3000

	retryExhaustedInterval   = 10
	retryUnavailableInterval = 100

	refreshMetricsInterval = time.Duration(10) * time.Second

	metricsRPCCalls        = "rpc_call_total"
	metricsRPCRetries      = "rpc_retries_total"
	metricsRPCFailures     = "rpc_error_total"
	metricsRPCPending      = "rpc_pending_num"
	metricsRPCCapacity     = "rpc_capacity_num"
	metricsGRPCActiveConns = "grpc_active_conn_num"
)

type grpcClientHelper struct {
	conn       *grpc.ClientConn
	recovering bool
	mu         sync.Mutex

	active int64
}

// Returns nil if connection in the READY/IDLE/CONNECTING state.
// If connection is in the TransientFailure state, we try to re-connect immediately
// once.
// See https://github.com/grpc/grpc/blob/master/doc/connectivity-semantics-and-api.md
// and https://github.com/grpc/grpc/blob/master/doc/connection-backoff.md
// See also https://github.com/cockroachdb/cockroach/blob/master/pkg/util/grpcutil/grpc_util.go
func (st *grpcClientHelper) Ready() error {
	s := st.conn.GetState()

	if s == connectivity.Shutdown {
		return errors.New("grpc connection is closed")
	}

	if s == connectivity.TransientFailure {
		return st.tryRecover()
	}

	if st.recovering {
		st.reset()
	}

	return nil
}

func (st *grpcClientHelper) Close() {
	st.conn.Close()
}

func (st *grpcClientHelper) ActiveConns() int {
	return int(atomic.LoadInt64(&st.active))
}

func (st *grpcClientHelper) SupportsActiveConns() bool {
	return true
}

func (st *grpcClientHelper) HandleConn(ctx context.Context, stat stats.ConnStats) {
	var addr string

	if p, ok := peer.FromContext(ctx); ok {
		addr = p.Addr.String()
	}

	if _, ok := stat.(*stats.ConnBegin); ok {
		log.WithField("context", "grpc").Debugf("connected to %s", addr)
		atomic.AddInt64(&st.active, 1)
	}

	if _, ok := stat.(*stats.ConnEnd); ok {
		log.WithField("context", "grpc").Debugf("disconnected from %s", addr)
		atomic.AddInt64(&st.active, -1)
	}
}

func (st *grpcClientHelper) HandleRPC(ctx context.Context, stat stats.RPCStats) {
	// no-op
}

func (st *grpcClientHelper) TagConn(ctx context.Context, stat *stats.ConnTagInfo) context.Context {
	return ctx
}

func (st *grpcClientHelper) TagRPC(ctx context.Context, stat *stats.RPCTagInfo) context.Context {
	return ctx
}

func (st *grpcClientHelper) tryRecover() error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.recovering {
		return errors.New("grpc connection is not ready")
	}

	st.recovering = true
	st.conn.ResetConnectBackoff()

	log.WithField("context", "rpc").Warn("Connection is lost. Trying to reconnect immediately")

	return nil
}

func (st *grpcClientHelper) reset() {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.recovering {
		st.recovering = false
		log.WithField("context", "rpc").Info("Connection is restored")
	}
}

// Controller implements node.Controller interface for gRPC
type Controller struct {
	config      *Config
	barrier     Barrier
	client      pb.RPCClient
	metrics     metrics.Instrumenter
	log         *log.Entry
	clientState ClientHelper

	timerMu      sync.Mutex
	metricsTimer *time.Timer
}

// NewController builds new Controller
func NewController(metrics metrics.Instrumenter, config *Config) *Controller {
	metrics.RegisterCounter(metricsRPCCalls, "The total number of RPC calls")
	metrics.RegisterCounter(metricsRPCRetries, "The total number of RPC call retries")
	metrics.RegisterCounter(metricsRPCFailures, "The total number of failed RPC calls")
	metrics.RegisterGauge(metricsRPCPending, "The number of pending RPC calls")

	capacity := config.Concurrency
	barrier := NewFixedSizeBarrier(capacity)

	if barrier.HasDynamicCapacity() {
		metrics.RegisterGauge(metricsRPCCapacity, "The max number of concurrent RPC calls allowed")
		metrics.GaugeSet(metricsRPCCapacity, uint64(barrier.Capacity()))
	}

	if config.DialFun == nil {
		metrics.RegisterGauge(metricsGRPCActiveConns, "The number of active HTTP connections used by gRPC")
	}

	return &Controller{log: log.WithField("context", "rpc"), metrics: metrics, config: config, barrier: barrier}
}

// Start initializes RPC connection pool
func (c *Controller) Start() error {
	host := c.config.Host
	enableTLS := c.config.EnableTLS

	var dialer Dialer

	if c.config.DialFun != nil {
		dialer = c.config.DialFun
	} else {
		dialer = defaultDialer
	}

	client, state, err := dialer(c.config)

	if err == nil {
		c.log.Infof("RPC controller initialized: %s (concurrency: %s, enable_tls: %t, proto_versions: %s)", host, c.barrier.CapacityInfo(), enableTLS, ProtoVersions)
	} else {
		return err
	}

	c.client = client
	c.clientState = state

	if c.barrier.HasDynamicCapacity() || state.SupportsActiveConns() {
		c.metricsTimer = time.AfterFunc(refreshMetricsInterval, c.refreshMetrics)
	}

	c.barrier.Start()

	return nil
}

// Shutdown closes connections
func (c *Controller) Shutdown() error {
	if c.clientState == nil {
		return nil
	}

	c.timerMu.Lock()
	if c.metricsTimer != nil {
		c.metricsTimer.Stop()
	}
	c.timerMu.Unlock()

	defer c.clientState.Close()

	busy := c.busy()

	if busy > 0 {
		c.log.Infof("Waiting for active RPC calls to finish: %d", busy)
	}

	// Wait for active connections
	_, err := c.retry("", func() (interface{}, error) {
		busy := c.busy()

		if busy > 0 {
			return false, fmt.Errorf("There are %d active RPC connections left", busy)
		}

		c.log.Info("All active RPC calls finished")
		return true, nil
	})

	c.barrier.Stop()

	return err
}

// Authenticate performs Connect RPC call
func (c *Controller) Authenticate(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	c.metrics.GaugeIncrement(metricsRPCPending)
	c.barrier.Acquire()
	c.metrics.GaugeDecrement(metricsRPCPending)

	defer c.barrier.Release()

	op := func() (interface{}, error) {
		return c.client.Connect(
			newContext(sid),
			protocol.NewConnectMessage(env),
		)
	}

	c.metrics.CounterIncrement(metricsRPCCalls)

	response, err := c.retry(sid, op)

	if err != nil {
		c.metrics.CounterIncrement(metricsRPCFailures)

		return nil, err
	}

	if r, ok := response.(*pb.ConnectionResponse); ok {

		c.log.WithField("sid", sid).Debugf("Authenticate response: %v", r)

		reply, err := protocol.ParseConnectResponse(r)

		return reply, err
	}

	c.metrics.CounterIncrement(metricsRPCFailures)

	return nil, errors.New("failed to deserialize connection response")
}

// Subscribe performs Command RPC call with "subscribe" command
func (c *Controller) Subscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	c.metrics.GaugeIncrement(metricsRPCPending)
	c.barrier.Acquire()
	c.metrics.GaugeDecrement(metricsRPCPending)

	defer c.barrier.Release()

	op := func() (interface{}, error) {
		return c.client.Command(
			newContext(sid),
			protocol.NewCommandMessage(env, "subscribe", channel, id, ""),
		)
	}

	response, err := c.retry(sid, op)

	return c.parseCommandResponse(sid, response, err)
}

// Unsubscribe performs Command RPC call with "unsubscribe" command
func (c *Controller) Unsubscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	c.metrics.GaugeIncrement(metricsRPCPending)
	c.barrier.Acquire()
	c.metrics.GaugeDecrement(metricsRPCPending)

	defer c.barrier.Release()

	op := func() (interface{}, error) {
		return c.client.Command(
			newContext(sid),
			protocol.NewCommandMessage(env, "unsubscribe", channel, id, ""),
		)
	}

	response, err := c.retry(sid, op)

	return c.parseCommandResponse(sid, response, err)
}

// Perform performs Command RPC call with "perform" command
func (c *Controller) Perform(sid string, env *common.SessionEnv, id string, channel string, data string) (*common.CommandResult, error) {
	c.metrics.GaugeIncrement(metricsRPCPending)
	c.barrier.Acquire()
	c.metrics.GaugeDecrement(metricsRPCPending)

	defer c.barrier.Release()

	op := func() (interface{}, error) {
		return c.client.Command(
			newContext(sid),
			protocol.NewCommandMessage(env, "message", channel, id, data),
		)
	}

	response, err := c.retry(sid, op)

	return c.parseCommandResponse(sid, response, err)
}

// Disconnect performs disconnect RPC call
func (c *Controller) Disconnect(sid string, env *common.SessionEnv, id string, subscriptions []string) error {
	c.metrics.GaugeIncrement(metricsRPCPending)
	c.barrier.Acquire()
	c.metrics.GaugeDecrement(metricsRPCPending)

	defer c.barrier.Release()

	op := func() (interface{}, error) {
		return c.client.Disconnect(
			newContext(sid),
			protocol.NewDisconnectMessage(env, id, subscriptions),
		)
	}

	c.metrics.CounterIncrement(metricsRPCCalls)

	response, err := c.retry(sid, op)

	if err != nil {
		c.metrics.CounterIncrement(metricsRPCFailures)
		return err
	}

	if r, ok := response.(*pb.DisconnectResponse); ok {
		c.log.WithField("sid", sid).Debugf("Disconnect response: %v", r)

		err = protocol.ParseDisconnectResponse(r)

		if err != nil {
			c.metrics.CounterIncrement(metricsRPCFailures)
		}

		return err
	}

	return errors.New("failed to deserialize disconnect response")
}

func (c *Controller) parseCommandResponse(sid string, response interface{}, err error) (*common.CommandResult, error) {
	c.metrics.CounterIncrement(metricsRPCCalls)

	if err != nil {
		c.metrics.CounterIncrement(metricsRPCFailures)

		return nil, err
	}

	if r, ok := response.(*pb.CommandResponse); ok {
		c.log.WithField("sid", sid).Debugf("Command response: %v", r)

		res, err := protocol.ParseCommandResponse(r)

		return res, err
	}

	c.metrics.CounterIncrement(metricsRPCFailures)

	return nil, errors.New("failed to deserialize command response")
}

func (c *Controller) busy() int {
	return c.barrier.BusyCount()
}

func (c *Controller) retry(sid string, callback func() (interface{}, error)) (res interface{}, err error) {
	retryAge := 0
	attempt := 0
	wasExhausted := false

	for {
		if stErr := c.clientState.Ready(); stErr != nil {
			return nil, stErr
		}

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

		c.log.WithFields(log.Fields{"sid": sid, "code": st.Code()}).Debugf("RPC failure: %v", st.Message())

		interval := retryUnavailableInterval

		if st.Code() == codes.ResourceExhausted {
			interval = retryExhaustedInterval
			if !wasExhausted {
				attempt = 0
				wasExhausted = true
			}
			c.barrier.Exhausted()
		} else if wasExhausted {
			wasExhausted = false
			attempt = 0
		}

		delayMS := int(math.Pow(2, float64(attempt))) * interval
		delay := time.Duration(delayMS)

		retryAge += delayMS

		c.metrics.CounterIncrement(metricsRPCRetries)

		time.Sleep(delay * time.Millisecond)

		attempt++
	}
}

func newContext(sessionID string) context.Context {
	md := metadata.Pairs("sid", sessionID, "protov", ProtoVersions)
	return metadata.NewOutgoingContext(context.Background(), md)
}

func defaultDialer(conf *Config) (pb.RPCClient, ClientHelper, error) {
	host := conf.Host
	enableTLS := conf.EnableTLS

	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		PermitWithoutStream: true,             // send pings even without active streams
	}

	const grpcServiceConfig = `{"loadBalancingPolicy":"round_robin"}`

	state := &grpcClientHelper{}

	dialOptions := []grpc.DialOption{
		grpc.WithKeepaliveParams(kacp),
		grpc.WithDefaultServiceConfig(grpcServiceConfig),
		grpc.WithStatsHandler(state),
	}

	if enableTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		}

		dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	var callOptions = []grpc.CallOption{}

	// Zero is the default
	if conf.MaxRecvSize != 0 {
		callOptions = append(callOptions, grpc.MaxCallRecvMsgSize(conf.MaxRecvSize))
	}

	if conf.MaxSendSize != 0 {
		callOptions = append(callOptions, grpc.MaxCallSendMsgSize(conf.MaxSendSize))
	}

	if len(callOptions) > 0 {
		dialOptions = append(dialOptions, grpc.WithDefaultCallOptions(callOptions...))
	}

	conn, err := grpc.Dial(
		host,
		dialOptions...,
	)

	if err != nil {
		return nil, nil, err
	}

	client := pb.NewRPCClient(conn)
	state.conn = conn

	return client, state, nil
}

func (c *Controller) refreshMetrics() {
	if c.clientState.SupportsActiveConns() {
		c.metrics.GaugeSet(metricsGRPCActiveConns, uint64(c.clientState.ActiveConns()))
	}

	if c.barrier.HasDynamicCapacity() {
		c.metrics.GaugeSet(metricsRPCCapacity, uint64(c.barrier.Capacity()))
	}

	c.timerMu.Lock()
	defer c.timerMu.Unlock()

	c.metricsTimer = time.AfterFunc(refreshMetricsInterval, c.refreshMetrics)
}
