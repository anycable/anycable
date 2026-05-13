// Package anycable is a WebSocket client for the AnyCable cable protocol.
//
// This package is a hard-fork of github.com/cinemast/anycable-go-client
// (https://github.com/cinemast/anycable-go-client) at commit
// ae4b0165762b21c52b47a28de913891a7478d973, originally located at
// pkg/anycable/. The upstream repository did not ship a LICENSE file at the
// time of fork; the source is preserved here under the original authorship of
// the cinemast project. Modifications made after the fork (synchronous
// Subscribe semantics, configurable per-subscription channel capacity, and
// other benchi-specific changes) land in subsequent implementation units —
// this file is the verbatim baseline.
package anycable

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// DefaultSubscriptionCapacity is the per-subscription Messages-channel
// capacity used when Client.SubscriptionCapacity is zero. Matches the
// cinemast upstream's hardcoded value so unmodified callers see the same
// behavior; benchi overrides this to a smaller value to bound memory.
const DefaultSubscriptionCapacity = 1000

type ChannelIdentifier interface {
	String() string
}

type Client struct {
	url    string
	logger *slog.Logger
	ws     *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc

	// SubscriptionCapacity sets the per-subscription Messages-channel
	// capacity for subscriptions created after the field is set. Zero falls
	// back to DefaultSubscriptionCapacity.
	SubscriptionCapacity int

	subscriptions map[string]*Subscription
	mutex         sync.Mutex
}

type Command struct {
	Command    string  `json:"command"`
	Identifier *string `json:"identifier,omitempty"`
	Data       *string `json:"data,omitempty"`
	Action     *string `json:"action,omitempty"`
}

type Event struct {
	Type       string          `json:"type"`
	Message    json.RawMessage `json:"message"`
	Identifier *string         `json:"identifier"`
}

func MessageToStruct[T any](e Event) (*T, error) {
	var r T
	err := json.Unmarshal(e.Message, &r)
	return &r, err
}

type Subscription struct {
	c          *Client
	Identifier ChannelIdentifier
	Subscribed bool
	Rejected   bool
	Messages   chan Event

	// done is closed by the read loop the first time the server resolves the
	// subscription — by emitting either `confirm_subscription` (Subscribed
	// becomes true) or `reject_subscription` (Rejected becomes true and
	// Messages is closed). SubscribeAndWait blocks on this channel.
	done       chan struct{}
	doneOnce   sync.Once
	closeOnce  sync.Once
}

func (s *Subscription) signalDone() {
	s.doneOnce.Do(func() { close(s.done) })
}

// closeMessages closes the Messages channel exactly once. Both the reject
// path and the connection-loss cleanup path call it; without the guard the
// cleanup loop would double-close any already-rejected subscription.
func (s *Subscription) closeMessages() {
	s.closeOnce.Do(func() { close(s.Messages) })
}

func NewClient(ctx context.Context, url string, logger *slog.Logger) *Client {
	subCtx, cancel := context.WithCancel(ctx)
	return &Client{
		url:           url,
		logger:        logger,
		ctx:           subCtx,
		cancel:        cancel,
		subscriptions: make(map[string]*Subscription),
	}
}

func (a *Client) Connect() error {
	var err error
	a.ws, _, err = websocket.DefaultDialer.Dial(a.url, nil)
	if err != nil {
		return fmt.Errorf("error connecting to AnyCable server: %w", err)
	}

	msg, err := a.readMessage()
	if err != nil {
		return err
	}

	if msg.Type != "welcome" {
		defer a.Close()
		return fmt.Errorf("unexpected message type: %s", msg.Type)
	}

	go func() {
		for {
			ev, err := a.readMessage()
			if err != nil {
				a.mutex.Lock()
				for _, sub := range a.subscriptions {
					sub.closeMessages()
					sub.signalDone()
				}
				a.mutex.Unlock()
				break
			}
			switch ev.Type {
			case "ping":
				err := a.sendCommand(Command{Command: "pong"})
				if err != nil {
					a.logger.Error("error writing to AnyCable server", "err", err)
				}
			case "disconnect":
				a.logger.Debug("disconnected from AnyCable server", "ev", ev)
				err = a.Close()
				if err != nil {
					a.logger.Error("error closing AnyCable client", "err", err)
					return
				}
			case "reject_subscription":
				a.mutex.Lock()
				sub, ok := a.subscriptions[*ev.Identifier]
				a.mutex.Unlock()
				if !ok {
					a.logger.Warn("received reject_subscription for unknown subscription", "identifier", ev.Identifier)
					continue
				}
				sub.Rejected = true
				sub.closeMessages()
				sub.signalDone()
			case "confirm_subscription":
				a.mutex.Lock()
				sub, ok := a.subscriptions[*ev.Identifier]
				a.mutex.Unlock()
				if !ok {
					a.logger.Warn("received confirm_subscription for unknown subscription", "identifier", ev.Identifier)
					continue
				}
				sub.Subscribed = true
				sub.signalDone()
			default:
				if ev.Identifier == nil {
					a.logger.Warn("received unknown message", "ev", ev)
					continue
				}
				a.mutex.Lock()
				sub, ok := a.subscriptions[*ev.Identifier]
				a.mutex.Unlock()
				if !ok {
					a.logger.Warn("received message for unknown subscription", "identifier", ev.Identifier)
				}
				sub.Messages <- *ev
			}
		}
	}()
	return nil
}

func (a *Client) Subscribe(identifier ChannelIdentifier) (*Subscription, error) {
	capacity := a.SubscriptionCapacity
	if capacity <= 0 {
		capacity = DefaultSubscriptionCapacity
	}
	sub := &Subscription{
		c:          a,
		Identifier: identifier,
		Messages:   make(chan Event, capacity),
		done:       make(chan struct{}),
	}
	id := identifier.String()
	a.mutex.Lock()
	a.subscriptions[id] = sub
	a.mutex.Unlock()

	err := a.sendCommand(Command{
		Command:    "subscribe",
		Identifier: &id,
	})
	if err != nil {
		a.mutex.Lock()
		delete(a.subscriptions, id)
		a.mutex.Unlock()
		return nil, err
	}
	return sub, nil
}

// SubscribeAndWait creates a subscription and blocks until the server confirms
// it, rejects it, the connection drops, or the timeout elapses. On
// confirmation it returns the live subscription. On rejection, timeout, or
// disconnect it returns a non-nil error naming the identifier.
func (a *Client) SubscribeAndWait(identifier ChannelIdentifier, timeout time.Duration) (*Subscription, error) {
	sub, err := a.Subscribe(identifier)
	if err != nil {
		return nil, err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-sub.done:
		if sub.Rejected {
			return nil, fmt.Errorf("subscription rejected: %s", identifier.String())
		}
		if !sub.Subscribed {
			return nil, fmt.Errorf("subscription closed before confirmation: %s", identifier.String())
		}
		return sub, nil
	case <-timer.C:
		return nil, fmt.Errorf("subscription confirmation timed out: %s", identifier.String())
	}
}

func (a *Client) Unsubscribe(subscription *Subscription) error {
	a.mutex.Lock()
	delete(a.subscriptions, subscription.Identifier.String())
	a.mutex.Unlock()

	id := subscription.Identifier.String()

	return a.sendCommand(Command{
		Command:    "unsubscribe",
		Identifier: &id,
	})
}

func (a *Client) Send(identifier ChannelIdentifier, message any) error {
	return a.sendMessage(identifier, message)
}

func (a *Client) sendMessage(identifier ChannelIdentifier, message any) error {

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("error encoding command: %w", err)
	}
	txt := string(data)

	id := identifier.String()

	return a.sendCommand(Command{
		Command:    "message",
		Identifier: &id,
		Data:       &txt,
	})
}

func (a *Client) Close() error {
	a.logger.Debug("anycable: closing connection")
	a.cancel()
	if a.ws == nil {
		return nil
	}
	return a.ws.Close()
}

func (a *Client) sendCommand(c Command) error {
	text, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("error encoding command: %w", err)
	}
	return a.sendData(text)
}

func (a *Client) sendData(message []byte) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.logger.Debug("anycable<-" + string(message))
	return a.ws.WriteMessage(websocket.TextMessage, message)
}

func (a *Client) readMessage() (*Event, error) {
	ev := &Event{}

	t, msg, err := a.ws.ReadMessage()
	if err != nil {
		a.logger.Error("error reading message from AnyCable server", "err", err)
		return nil, fmt.Errorf("error reading message from AnyCable server: %w", err)
	}

	if t != websocket.TextMessage {
		return nil, fmt.Errorf("received unexpected message type: %d", t)
	}
	a.logger.Debug("anycable->" + string(msg))
	err = json.Unmarshal(msg, ev)
	if err != nil {
		return nil, fmt.Errorf("could not deserialize message %s: %w", string(msg), err)
	}
	return ev, nil
}
