package lib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	anycable "github.com/anycable/anycable-go/etc/benchi/client"
)

// Message is one server-pushed envelope on a subscribed stream after the
// envelope's identifier has been resolved back to its stream name.
type Message struct {
	Stream string
	Data   string
}

// streamIdentifier is the channel-identifier JSON shape AnyCable expects for
// public-streams subscriptions: `{"channel":"$pubsub","stream_name":"foo"}`.
type streamIdentifier struct {
	Channel    string `json:"channel"`
	StreamName string `json:"stream_name"`
}

func (s streamIdentifier) String() string {
	b, _ := json.Marshal(s)
	return string(b)
}

func pubsub(stream string) streamIdentifier {
	return streamIdentifier{Channel: "$pubsub", StreamName: stream}
}

const (
	defaultMergedCapacity       = 256
	defaultSubscriptionCapacity = 16
	defaultSubscribeTimeout     = 5 * time.Second
)

// Client is a benchi-flavored WebSocket subscriber: thin wrapper around the
// forked cinemast client that exposes a single fan-in Message channel and
// per-stream subscribe-with-confirm semantics.
type Client struct {
	inner             *anycable.Client
	subscribeTimeout  time.Duration
	merged            chan Message
	forwarders        sync.WaitGroup
	ctx               context.Context
	cancel            context.CancelFunc

	mu             sync.Mutex
	closed         bool
	subscribed     map[string]struct{}
}

// BuildClient constructs a Client targeting serverURL with a 16-deep
// per-subscription buffer and a 256-deep merged buffer. The client is not
// connected until Connect is called.
func BuildClient(serverURL string) (*Client, error) {
	if serverURL == "" {
		return nil, errors.New("server URL is required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	inner := anycable.NewClient(ctx, serverURL, logger)
	inner.SubscriptionCapacity = defaultSubscriptionCapacity
	return &Client{
		inner:            inner,
		subscribeTimeout: defaultSubscribeTimeout,
		merged:           make(chan Message, defaultMergedCapacity),
		ctx:              ctx,
		cancel:           cancel,
		subscribed:       make(map[string]struct{}),
	}, nil
}

// Connect dials the server and consumes the welcome envelope. Honors ctx for
// cancellation: if ctx is done before dial returns, the inner client is
// closed to unblock the in-flight dial.
func (c *Client) Connect(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.inner.Connect()
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		_ = c.inner.Close()
		<-errCh
		return ctx.Err()
	}
}

// Subscribe issues subscribe commands for each stream and waits for each
// server-side confirmation. Duplicate streams are deduped on the client side,
// so calling Subscribe with a stream that's already subscribed is a no-op
// (no second subscribe command goes on the wire). Returns an error naming
// every stream that timed out or was rejected.
func (c *Client) Subscribe(streams []string) error {
	var failed []string
	for _, stream := range streams {
		c.mu.Lock()
		if _, ok := c.subscribed[stream]; ok {
			c.mu.Unlock()
			continue
		}
		c.mu.Unlock()

		sub, err := c.inner.SubscribeAndWait(pubsub(stream), c.subscribeTimeout)
		if err != nil {
			failed = append(failed, stream)
			continue
		}

		c.mu.Lock()
		c.subscribed[stream] = struct{}{}
		c.mu.Unlock()

		c.startForwarder(sub, stream)
	}
	if len(failed) > 0 {
		return fmt.Errorf("subscribe failed for streams: %v", failed)
	}
	return nil
}

func (c *Client) startForwarder(sub *anycable.Subscription, stream string) {
	c.forwarders.Add(1)
	go func() {
		defer c.forwarders.Done()
		for ev := range sub.Messages {
			var data string
			if err := json.Unmarshal(ev.Message, &data); err != nil {
				// Non-string payload — surface the raw JSON so the caller can
				// still distinguish broadcasts (e.g., JSON objects from
				// application-shaped data).
				data = string(ev.Message)
			}
			select {
			case c.merged <- Message{Stream: stream, Data: data}:
			case <-c.ctx.Done():
				return
			}
		}
	}()
}

// Receive blocks until the next inbound message or until Close has been
// called and all in-flight messages have drained. Returns (Message{}, false)
// when the client has been closed and no more messages will arrive.
func (c *Client) Receive() (Message, bool) {
	msg, ok := <-c.merged
	return msg, ok
}

// Close stops the inner connection, waits for forwarder goroutines to drain,
// and closes the merged channel. Safe to call more than once.
func (c *Client) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.mu.Unlock()

	_ = c.inner.Close()
	c.cancel()
	c.forwarders.Wait()
	close(c.merged)
}
