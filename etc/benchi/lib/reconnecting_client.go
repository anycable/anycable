package lib

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// ClientState is the lifecycle phase of a ReconnectingClient.
type ClientState int

const (
	// StateConnecting: dialing the socket or waiting for the welcome message.
	StateConnecting ClientState = iota
	// StateSubscribing: welcome received, waiting for subscription confirmations.
	StateSubscribing
	// StateHealthy: all subscriptions confirmed.
	StateHealthy
	// StateBackoff: waiting before the next connection attempt.
	StateBackoff
	clientStatesNum
)

func (s ClientState) String() string {
	switch s {
	case StateConnecting:
		return "connecting"
	case StateSubscribing:
		return "subscribing"
	case StateHealthy:
		return "healthy"
	case StateBackoff:
		return "backoff"
	}
	return "unknown"
}

// CloseReason explains why a connection attempt ended.
type CloseReason int

const (
	ReasonDialError CloseReason = iota
	ReasonPingTimeout
	ReasonSubscribeTimeout
	ReasonRejected
	ReasonServerDisconnect
	ReasonServerClosed
	ReasonDropped
	closeReasonsNum
)

func (r CloseReason) String() string {
	switch r {
	case ReasonDialError:
		return "dial_error"
	case ReasonPingTimeout:
		return "ping_timeout"
	case ReasonSubscribeTimeout:
		return "subscribe_timeout"
	case ReasonRejected:
		return "rejected"
	case ReasonServerDisconnect:
		return "server_disconnect"
	case ReasonServerClosed:
		return "server_closed"
	case ReasonDropped:
		return "dropped"
	}
	return "unknown"
}

// AllCloseReasons lists every CloseReason in display order.
func AllCloseReasons() []CloseReason {
	out := make([]CloseReason, 0, closeReasonsNum)
	for r := CloseReason(0); r < closeReasonsNum; r++ {
		out = append(out, r)
	}
	return out
}

// ClientStats aggregates state and timing across many ReconnectingClients.
// All methods are safe for concurrent use.
type ClientStats struct {
	states   [clientStatesNum]atomic.Int64
	reasons  [closeReasonsNum]atomic.Int64
	attempts atomic.Int64
	started  atomic.Int64
	retries  atomic.Int64

	// Welcome is the time from dial start to the welcome message.
	Welcome Histogram
	// Confirm is the time from the first subscribe command for a channel
	// to its confirmation (retries included).
	Confirm Histogram
	// Healthy is the time from dial start to the last confirmation.
	Healthy Histogram
}

// State returns the number of clients currently in state s.
func (cs *ClientStats) State(s ClientState) int64 { return cs.states[s].Load() }

// Reason returns the number of attempts that ended with reason r.
func (cs *ClientStats) Reason(r CloseReason) int64 { return cs.reasons[r].Load() }

// Attempts returns the total number of connection attempts (initial
// connects included).
func (cs *ClientStats) Attempts() int64 { return cs.attempts.Load() }

// Reconnects returns the number of connection attempts after each client's
// first one.
func (cs *ClientStats) Reconnects() int64 { return cs.attempts.Load() - cs.started.Load() }

// SubscribeRetries returns the number of re-sent subscribe commands.
func (cs *ClientStats) SubscribeRetries() int64 { return cs.retries.Load() }

// Backoff is a capped exponential backoff with ±50% jitter:
// delay(n) = min(Max, Min × 2^n) × U(0.5, 1.5), where n is the number of
// consecutive failed attempts. A client that reaches the healthy state
// resets n to zero.
type Backoff struct {
	Min time.Duration
	Max time.Duration
}

func (b Backoff) delay(n int, rng *rand.Rand) time.Duration {
	if b.Min <= 0 {
		return 0
	}
	d := b.Min
	for i := 0; i < n && d < b.Max; i++ {
		d *= 2
	}
	if b.Max > 0 && d > b.Max {
		d = b.Max
	}
	return time.Duration(float64(d) * (0.5 + rng.Float64()))
}

// ReconnectingClientConfig configures a ReconnectingClient.
type ReconnectingClientConfig struct {
	// URL is the WebSocket endpoint.
	URL string
	// Channels are the channel identifiers (raw JSON strings) to subscribe
	// to after each welcome.
	Channels []string
	// SequentialSubscribe sends the next subscribe command only after the
	// previous one is confirmed. Otherwise all commands go out at once.
	SequentialSubscribe bool
	// SubscribeRetryInterval is the wait for a confirmation before the
	// subscribe command is sent again. Must be positive.
	SubscribeRetryInterval time.Duration
	// SubscribeRetriesMax is the number of re-sends per channel. When the
	// last one is not confirmed in SubscribeRetryInterval, the client
	// reconnects.
	SubscribeRetriesMax int
	// PingTimeout closes the connection when no ping arrives for this long.
	// The watchdog starts when the socket opens.
	PingTimeout time.Duration
	// Backoff controls the delay between attempts.
	Backoff Backoff
	// Stats receives state transitions, close reasons, and timings.
	Stats *ClientStats
	// Seed seeds the backoff jitter.
	Seed uint64
}

// ReconnectingClient models a browser-like Action Cable client: it connects,
// subscribes to its channels, watches server pings, and reconnects with
// backoff when the ping deadline is missed, a subscription is not confirmed
// after all retries, or the connection is lost.
type ReconnectingClient struct {
	cfg   ReconnectingClientConfig
	rng   *rand.Rand
	state ClientState

	mu   sync.Mutex
	conn *websocket.Conn
	drop bool
}

// NewReconnectingClient builds a client. It does nothing until Run.
func NewReconnectingClient(cfg ReconnectingClientConfig) *ReconnectingClient {
	c := &ReconnectingClient{
		cfg:   cfg,
		rng:   rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x9E3779B97F4A7C15)),
		state: StateBackoff,
	}
	cfg.Stats.states[StateBackoff].Add(1)
	return c
}

// setState moves the client to s and updates the shared state counters.
// Only the Run goroutine calls it.
func (c *ReconnectingClient) setState(s ClientState) {
	if c.state == s {
		return
	}
	c.cfg.Stats.states[c.state].Add(-1)
	c.cfg.Stats.states[s].Add(1)
	c.state = s
}

// Drop closes the current connection abruptly (no close frame), as an LB
// restart or a network failure would. The client then reconnects with
// backoff. No-op when there is no open connection.
func (c *ReconnectingClient) Drop() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return false
	}
	c.drop = true
	_ = c.conn.UnderlyingConn().Close()
	return true
}

// Run executes the connect loop until ctx is done. initialDelay staggers
// the first attempt.
func (c *ReconnectingClient) Run(ctx context.Context, initialDelay time.Duration) {
	defer func() {
		c.cfg.Stats.states[c.state].Add(-1)
	}()

	if !sleepCtx(ctx, initialDelay) {
		return
	}
	c.cfg.Stats.started.Add(1)

	failures := 0
	for ctx.Err() == nil {
		healthy, reason := c.attempt(ctx)
		if ctx.Err() != nil {
			return
		}
		c.cfg.Stats.reasons[reason].Add(1)
		if healthy {
			failures = 0
		}
		c.setState(StateBackoff)
		if !sleepCtx(ctx, c.cfg.Backoff.delay(failures, c.rng)) {
			return
		}
		failures++
	}
}

type clientEvent struct {
	kind       string
	identifier string
	err        error
}

// attempt runs one connection from dial to close. healthy reports whether
// the client reached the healthy state during this attempt.
func (c *ReconnectingClient) attempt(ctx context.Context) (healthy bool, reason CloseReason) {
	stats := c.cfg.Stats
	stats.attempts.Add(1)
	c.setState(StateConnecting)
	start := time.Now()

	dialer := websocket.Dialer{HandshakeTimeout: c.cfg.PingTimeout}
	conn, _, err := dialer.DialContext(ctx, c.cfg.URL, nil)
	if err != nil {
		return false, ReasonDialError
	}

	c.mu.Lock()
	c.conn = conn
	c.drop = false
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		dropped := c.drop
		c.conn = nil
		c.mu.Unlock()
		if dropped {
			reason = ReasonDropped
		}
		_ = conn.Close()
	}()

	// The ping watchdog starts when the socket opens: each ping pushes the
	// read deadline forward; any read past the deadline fails.
	_ = conn.SetReadDeadline(time.Now().Add(c.cfg.PingTimeout))

	events := make(chan clientEvent, len(c.cfg.Channels)+4)
	stop := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		c.readLoop(conn, events, stop)
	}()
	defer func() {
		close(stop)
		_ = conn.UnderlyingConn().Close()
		<-readerDone
	}()

	// subscription tracks one channel's subscribe command until it is
	// confirmed.
	type subscription struct {
		sentAt  time.Time // the first send; confirmation latency counts from it
		retryAt time.Time // when to re-send (or give up)
		retries int
	}

	var (
		retryTimer *time.Timer
		retryC     <-chan time.Time
		pending    map[string]*subscription
		unsent     []string
		ctxDone    = ctx.Done()
		subscribed = false
	)
	defer func() {
		if retryTimer != nil {
			retryTimer.Stop()
		}
	}()

	subscribe := func(id string) error {
		now := time.Now()
		pending[id] = &subscription{sentAt: now, retryAt: now.Add(c.cfg.SubscribeRetryInterval)}
		return c.send(conn, "subscribe", id)
	}

	// armRetry points the retry timer at the earliest pending deadline.
	armRetry := func() {
		var next time.Time
		for _, sub := range pending {
			if next.IsZero() || sub.retryAt.Before(next) {
				next = sub.retryAt
			}
		}
		if next.IsZero() {
			retryC = nil
			return
		}
		if retryTimer == nil {
			retryTimer = time.NewTimer(time.Until(next))
		} else {
			retryTimer.Reset(time.Until(next))
		}
		retryC = retryTimer.C
	}

	for {
		select {
		case <-ctxDone:
			return false, ReasonDropped
		case <-retryC:
			now := time.Now()
			for id, sub := range pending {
				if now.Before(sub.retryAt) {
					continue
				}
				if sub.retries >= c.cfg.SubscribeRetriesMax {
					c.closeGracefully(conn)
					return healthy, ReasonSubscribeTimeout
				}
				sub.retries++
				sub.retryAt = now.Add(c.cfg.SubscribeRetryInterval)
				stats.retries.Add(1)
				if err := c.send(conn, "subscribe", id); err != nil {
					return healthy, ReasonServerClosed
				}
			}
			armRetry()
		case ev := <-events:
			switch ev.kind {
			case "welcome":
				if subscribed {
					continue
				}
				subscribed = true
				stats.Welcome.Record(time.Since(start))
				c.setState(StateSubscribing)
				pending = make(map[string]*subscription, len(c.cfg.Channels))
				unsent = c.cfg.Channels
				for len(unsent) > 0 {
					id := unsent[0]
					unsent = unsent[1:]
					if err := subscribe(id); err != nil {
						return false, ReasonServerClosed
					}
					if c.cfg.SequentialSubscribe {
						break
					}
				}
				if len(pending) == 0 {
					stats.Healthy.Record(time.Since(start))
					c.setState(StateHealthy)
					healthy = true
					continue
				}
				armRetry()
			case "confirm":
				sub, ok := pending[ev.identifier]
				if !ok {
					// A late confirmation for a re-sent command
					continue
				}
				delete(pending, ev.identifier)
				stats.Confirm.Record(time.Since(sub.sentAt))
				if len(unsent) > 0 {
					id := unsent[0]
					unsent = unsent[1:]
					if err := subscribe(id); err != nil {
						return healthy, ReasonServerClosed
					}
				}
				if len(pending) == 0 {
					stats.Healthy.Record(time.Since(start))
					c.setState(StateHealthy)
					healthy = true
				}
				armRetry()
			case "reject":
				c.closeGracefully(conn)
				return healthy, ReasonRejected
			case "disconnect":
				return healthy, ReasonServerDisconnect
			case "error":
				var ne net.Error
				if errors.As(ev.err, &ne) && ne.Timeout() {
					return healthy, ReasonPingTimeout
				}
				return healthy, ReasonServerClosed
			}
		}
	}
}

func (c *ReconnectingClient) readLoop(conn *websocket.Conn, events chan<- clientEvent, stop <-chan struct{}) {
	emit := func(ev clientEvent) bool {
		select {
		case events <- ev:
			return true
		case <-stop:
			return false
		}
	}
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			emit(clientEvent{kind: "error", err: err})
			return
		}
		var msg struct {
			Type       string `json:"type"`
			Identifier string `json:"identifier"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.Type {
		case "ping":
			_ = conn.SetReadDeadline(time.Now().Add(c.cfg.PingTimeout))
		case "welcome":
			if !emit(clientEvent{kind: "welcome"}) {
				return
			}
		case "confirm_subscription":
			if !emit(clientEvent{kind: "confirm", identifier: msg.Identifier}) {
				return
			}
		case "reject_subscription":
			if !emit(clientEvent{kind: "reject", identifier: msg.Identifier}) {
				return
			}
		case "disconnect":
			emit(clientEvent{kind: "disconnect"})
			return
		}
	}
}

func (c *ReconnectingClient) send(conn *websocket.Conn, command, identifier string) error {
	b, _ := json.Marshal(map[string]string{"command": command, "identifier": identifier})
	_ = conn.SetWriteDeadline(time.Now().Add(c.cfg.PingTimeout))
	return conn.WriteMessage(websocket.TextMessage, b)
}

// closeGracefully sends a close frame, as a browser's ws.close() would.
func (c *ReconnectingClient) closeGracefully(conn *websocket.Conn) {
	_ = conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(time.Second))
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
