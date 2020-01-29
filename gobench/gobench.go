// Package gobench implements alternative controller for benchmarking Go server w/o RPC.
// Mimics BenchmarkChannel from https://github.com/palkan/websocket-shootout/blob/master/ruby/action-cable-server/app/channels/benchmark_channel.rb
package gobench

import (
	"encoding/json"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/apex/log"

	nanoid "github.com/matoous/go-nanoid"
)

const (
	metricsCalls = "gochannels_call_total"

	identifier = "\"{\\\"channel\\\":\\\"BenchmarkChannel\\\"}\""

	welcomeMessage      = "{\"type\":\"welcome\"}"
	confirmationMessage = "{\"type\":\"confirm_subscription\",\"identifier\":\"{\\\"channel\\\":\\\"BenchmarkChannel\\\"}\"}"
)

// Identifiers represents a connection identifiers
type Identifiers struct {
	ID string `json:"id"`
}

// BroadcastMessage represents a pubsub payload
type BroadcastMessage struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

// Controller implements node.Controller interface for gRPC
type Controller struct {
	metrics *metrics.Metrics
	log     *log.Entry
}

// NewController builds new Controller from config
func NewController(metrics *metrics.Metrics) *Controller {
	metrics.RegisterCounter(metricsCalls, "The total number of Go channels calls")

	return &Controller{log: log.WithField("context", "gobench"), metrics: metrics}
}

// Start is no-op
func (c *Controller) Start() error {
	return nil
}

// Shutdown is no-op
func (c *Controller) Shutdown() error {
	return nil
}

// Authenticate allows everyone to connect and returns welcome message and rendom ID as identifier
func (c *Controller) Authenticate(sid string, env *common.SessionEnv) (string, []string, error) {
	c.metrics.Counter(metricsCalls).Inc()

	id, err := nanoid.Nanoid()

	if err != nil {
		return "", nil, err
	}

	identifiers := Identifiers{ID: id}
	idstr, err := json.Marshal(&identifiers)

	if err != nil {
		return "", nil, err
	}

	return string(idstr), []string{welcomeMessage}, nil
}

// Subscribe performs Command RPC call with "subscribe" command
func (c *Controller) Subscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	c.metrics.Counter(metricsCalls).Inc()
	res := &common.CommandResult{
		Disconnect:     false,
		StopAllStreams: false,
		Streams:        []string{"all"},
		Transmissions:  []string{confirmationMessage},
	}
	return res, nil
}

// Unsubscribe performs Command RPC call with "unsubscribe" command
func (c *Controller) Unsubscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	c.metrics.Counter(metricsCalls).Inc()
	res := &common.CommandResult{
		Disconnect:     false,
		StopAllStreams: true,
		Streams:        nil,
		Transmissions:  nil,
	}
	return res, nil
}

// Perform performs Command RPC call with "perform" command
func (c *Controller) Perform(sid string, env *common.SessionEnv, id string, channel string, data string) (res *common.CommandResult, err error) {
	c.metrics.Counter(metricsCalls).Inc()

	var payload map[string]interface{}

	if err = json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, err
	}

	switch action := payload["action"].(string); action {
	case "echo":
		res = &common.CommandResult{
			Disconnect:     false,
			StopAllStreams: false,
			Streams:        nil,
			Transmissions:  []string{string(data)},
		}
	case "broadcast":
		broadcastMsg, err := json.Marshal(&payload)

		if err != nil {
			return nil, err
		}

		broadcast := common.StreamMessage{
			Stream: "all",
			Data:   string(broadcastMsg),
		}

		payload["action"] = "broadcastResult"

		response, err := json.Marshal(
			map[string]interface{}{
				"message":    payload,
				"identifier": identifier,
			},
		)

		if err != nil {
			return nil, err
		}

		res = &common.CommandResult{
			Disconnect:     false,
			StopAllStreams: false,
			Streams:        nil,
			Transmissions:  []string{string(response)},
			Broadcasts:     []*common.StreamMessage{&broadcast},
		}
	default:
		res = &common.CommandResult{
			Disconnect:     false,
			StopAllStreams: false,
			Streams:        nil,
			Transmissions:  nil,
		}
	}

	return res, nil
}

// Disconnect performs disconnect RPC call
func (c *Controller) Disconnect(sid string, env *common.SessionEnv, id string, subscriptions []string) error {
	c.metrics.Counter(metricsCalls).Inc()
	return nil
}
