package rpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/protocol"
	"github.com/anycable/anycable-go/utils"
	"github.com/joomcode/errorx"

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

	retryExhaustedInterval   = int64(10)
	retryUnavailableInterval = int64(100)

	refreshMetricsInterval = time.Duration(10) * time.Second

	metricsRPCCalls        = "rpc_call_total"
	metricsRPCRetries      = "rpc_retries_total"
	metricsRPCFailures     = "rpc_error_total"
	metricsRPCTimeouts     = "rpc_timeout_total"
	metricsRPCPending      = "rpc_pending_num"
	metricsRPCCapacity     = "rpc_capacity_num"
	metricsGRPCActiveConns = "grpc_active_conn_num"

	secretKeyPhrase = "rpc-cable"
)

type grpcClientHelper struct {
	conn       *grpc.ClientConn
	recovering bool
	mu         sync.Mutex

	log    *slog.Logger
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
		st.log.Debug("connected", "addr", addr)
		atomic.AddInt64(&st.active, 1)
	}

	if _, ok := stat.(*stats.ConnEnd); ok {
		st.log.Debug("disconnected", "addr", addr)
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

	st.log.Warn("connection is lost, trying to reconnect immediately")

	return nil
}

func (st *grpcClientHelper) reset() {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.recovering {
		st.recovering = false
		st.log.Info("connection is restored")
	}
}

// Controller implements node.Controller interface for gRPC
type Controller struct {
	config      *Config
	barrier     Barrier
	client      pb.RPCClient
	metrics     metrics.Instrumenter
	log         *slog.Logger
	clientState ClientHelper

	// Timeout for a single RPC request
	requestTimeout time.Duration
	// Timeout for an RPC command that may retry a few times
	commandTimeout time.Duration

	timerMu      sync.Mutex
	metricsTimer *time.Timer
}

// NewController builds new Controller
func NewController(metrics metrics.Instrumenter, config *Config, l *slog.Logger) (*Controller, error) {
	metrics.RegisterCounter(metricsRPCCalls, "The total number of RPC calls")
	metrics.RegisterCounter(metricsRPCRetries, "The total number of RPC call retries")
	metrics.RegisterCounter(metricsRPCFailures, "The total number of failed RPC calls")
	metrics.RegisterCounter(metricsRPCTimeouts, "The total number of RPC call that timed out")
	metrics.RegisterGauge(metricsRPCPending, "The number of pending RPC calls")

	capacity := config.Concurrency
	if capacity <= 0 {
		capacity = defaultRPCConcurrency
		l.Warn("RPC concurrency must be positive, reverted to the default value")
	}
	barrier, err := NewFixedSizeBarrier(capacity)

	if err != nil {
		return nil, err
	}

	if barrier.HasDynamicCapacity() {
		metrics.RegisterGauge(metricsRPCCapacity, "The max number of concurrent RPC calls allowed")
		metrics.GaugeSet(metricsRPCCapacity, uint64(barrier.Capacity()))
	}

	if config.Impl() == "grpc" {
		metrics.RegisterGauge(metricsGRPCActiveConns, "The number of active HTTP connections used by gRPC")
	}

	requestTimeout := time.Duration(config.RequestTimeout) * time.Millisecond

	// TODO(v2): remove this fallback
	if config.Impl() == "http" {
		requestTimeout = time.Duration(config.GetHTTPRequestTimeout()) * time.Millisecond
	}

	commandTimeout := time.Duration(config.CommandTimeout) * time.Millisecond

	return &Controller{
		log:            l.With("context", "rpc"),
		metrics:        metrics,
		config:         config,
		barrier:        barrier,
		requestTimeout: requestTimeout,
		commandTimeout: commandTimeout,
	}, nil
}

// Start initializes RPC connection pool
func (c *Controller) Start() error {
	host := c.config.Host
	enableTLS := c.config.TLSEnabled()
	impl := c.config.Impl()

	dialer := c.config.DialFun

	if dialer == nil {
		switch impl {
		case "http":
			var err error

			if c.config.Secret == "" && c.config.SecretBase != "" {
				secret, verr := utils.NewMessageVerifier(c.config.SecretBase).Sign([]byte(secretKeyPhrase))

				if verr != nil {
					verr = errorx.Decorate(verr, "failed to auto-generate authentication key for HTTP RPC")
					return verr
				}

				c.log.Info("auto-generated authorization secret from the application secret")
				c.config.Secret = string(secret)
			}

			dialer, err = NewHTTPDialer(c.config)
			if err != nil {
				return err
			}
		case "grpc":
			dialer = defaultDialer
		default:
			return fmt.Errorf("unknown RPC implementation: %s", impl)
		}
	}

	client, state, err := dialer(c.config, c.log)

	if err == nil {
		proxiedHeaders := strings.Join(c.config.ProxyHeaders, ",")
		if proxiedHeaders == "" {
			proxiedHeaders = "<none>"
		}
		proxiedCookies := strings.Join(c.config.ProxyCookies, ",")
		if proxiedCookies == "" {
			proxiedCookies = "<all>"
		}
		c.log.Info(fmt.Sprintf("RPC controller initialized: %s (concurrency: %s, impl: %s, enable_tls: %t, proto_versions: %s, proxy_headers: %s, proxy_cookies: %s)", host, c.barrier.CapacityInfo(), impl, enableTLS, ProtoVersions, proxiedHeaders, proxiedCookies))
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
		c.log.Info("waiting for active RPC calls to finish", "num", busy)
	}

	// Wait for active connections
	_, err := c.retry("", func(ctx context.Context) (interface{}, error) {
		busy := c.busy()

		if busy > 0 {
			return false, fmt.Errorf("terminated while completing active RPC calls: %d", busy)
		}

		c.log.Info("all active RPC calls finished")
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

	op := func(ctx context.Context) (interface{}, error) {
		ctx, cancel := c.newRequestContext(ctx, sid)
		defer cancel()

		return c.client.Connect(ctx, protocol.NewConnectMessage(env))
	}

	c.metrics.CounterIncrement(metricsRPCCalls)

	response, err := c.retry(sid, op)

	if err != nil {
		c.metrics.CounterIncrement(metricsRPCFailures)

		return nil, err
	}

	if r, ok := response.(*pb.ConnectionResponse); ok {
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

	op := func(ctx context.Context) (interface{}, error) {
		ctx, cancel := c.newRequestContext(ctx, sid)
		defer cancel()

		return c.client.Command(
			ctx,
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

	op := func(ctx context.Context) (interface{}, error) {
		ctx, cancel := c.newRequestContext(ctx, sid)
		defer cancel()

		return c.client.Command(
			ctx,
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

	op := func(ctx context.Context) (interface{}, error) {
		ctx, cancel := c.newRequestContext(ctx, sid)
		defer cancel()

		return c.client.Command(
			ctx,
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

	op := func(ctx context.Context) (interface{}, error) {
		ctx, cancel := c.newRequestContext(ctx, sid)
		defer cancel()

		return c.client.Disconnect(
			ctx,
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
		res, err := protocol.ParseCommandResponse(r)

		return res, err
	}

	c.metrics.CounterIncrement(metricsRPCFailures)

	return nil, errors.New("failed to deserialize command response")
}

func (c *Controller) busy() int {
	return c.barrier.BusyCount()
}

func (c *Controller) retry(sid string, callback func(context.Context) (interface{}, error)) (res interface{}, err error) {
	attempt := 0
	wasExhausted := false
	var ctx context.Context

	if c.commandTimeout > 0 {
		ctx_, cancel := context.WithTimeout(
			context.Background(),
			c.commandTimeout,
		)
		ctx = ctx_
		defer cancel()
	} else {
		ctx = context.Background()
	}

	for {
		if stErr := c.clientState.Ready(); stErr != nil {
			return nil, stErr
		}

		res, err = callback(ctx)

		if err == nil {
			return res, nil
		}

		st, ok := status.FromError(err)
		if !ok {
			return nil, err
		}

		code := st.Code()

		if code == codes.DeadlineExceeded {
			c.metrics.CounterIncrement(metricsRPCTimeouts)
		}

		if !(code == codes.ResourceExhausted || code == codes.Unavailable) {
			return nil, err
		}

		c.log.With("sid", sid).Debug("RPC failed", "code", st.Code(), "error", st.Message())

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

		delayMS := int64(math.Pow(2, float64(attempt))) * interval
		delay := time.Duration(delayMS)

		c.metrics.CounterIncrement(metricsRPCRetries)

		select {
		case <-time.After(delay * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		attempt++
	}
}

func noopCancel() {}

func (c *Controller) newRequestContext(parentCtx context.Context, sessionID string) (ctx context.Context, cancel context.CancelFunc) {
	if c.requestTimeout > 0 {
		ctx, cancel = context.WithTimeout(parentCtx, c.requestTimeout)
	} else {
		ctx = parentCtx
		cancel = noopCancel
	}

	md := metadata.Pairs("sid", sessionID, "protov", ProtoVersions)
	ctx = metadata.NewOutgoingContext(ctx, md)
	return
}

func defaultDialer(conf *Config, l *slog.Logger) (pb.RPCClient, ClientHelper, error) {
	host := conf.Host
	enableTLS := conf.TLSEnabled()

	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		PermitWithoutStream: true,             // send pings even without active streams
	}

	const grpcServiceConfig = `{"loadBalancingConfig": [{"round_robin":{}}]}`

	state := &grpcClientHelper{log: l.With("impl", "grpc")}

	dialOptions := []grpc.DialOption{
		grpc.WithKeepaliveParams(kacp),
		grpc.WithDefaultServiceConfig(grpcServiceConfig),
		grpc.WithStatsHandler(state),
	}

	if enableTLS {
		tlsConfig, error := conf.TLSConfig()
		if error != nil {
			return nil, nil, error
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

	conn, err := grpc.NewClient(
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
