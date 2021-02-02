package rpc

import (
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	pb "github.com/anycable/anycable-go/protos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func NewTestController() *Controller {
	config := NewConfig()
	controller := NewController(metrics.NewMetrics(nil, 0), &config)
	controller.initSemaphore(1)
	return controller
}

func TestAuthenticate(t *testing.T) {
	controller := NewTestController()
	client := mocks.RPCClient{}
	controller.client = &client

	t.Run("Success", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=secret;"}

		client.On("Connect", mock.Anything,
			&pb.ConnectionRequest{
				Path:    url,
				Headers: headers,
				Env:     &pb.Env{Url: url, Headers: headers},
			}).Return(
			&pb.ConnectionResponse{
				Identifiers:   "user=john",
				Transmissions: []string{"welcome"},
				Status:        pb.Status_SUCCESS,
				Env:           &pb.EnvResponse{Cstate: map[string]string{"_s_": "test-session"}},
			}, nil)

		res, err := controller.Authenticate("42", &common.SessionEnv{URL: url, Headers: &headers})
		assert.Nil(t, err)
		assert.Equal(t, []string{"welcome"}, res.Transmissions)
		assert.Equal(t, "user=john", res.Identifier)
		assert.Equal(t, map[string]string{"_s_": "test-session"}, res.CState)
		assert.Empty(t, res.Broadcasts)
	})

	t.Run("Failure", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=invalid;"}

		client.On("Connect", mock.Anything,
			&pb.ConnectionRequest{
				Path:    url,
				Headers: headers,
				Env:     &pb.Env{Url: url, Headers: headers},
			}).Return(
			&pb.ConnectionResponse{
				Transmissions: []string{"unauthorized"},
				Status:        pb.Status_FAILURE,
				Env:           &pb.EnvResponse{Cstate: map[string]string{"_s_": "test-session"}},
				ErrorMsg:      "Authentication failed",
			}, nil)

		res, err := controller.Authenticate("42", &common.SessionEnv{URL: url, Headers: &headers})
		assert.NotNil(t, err)
		assert.Error(t, err, "Authentication failed")
		assert.Equal(t, []string{"unauthorized"}, res.Transmissions)
		assert.Equal(t, "", res.Identifier)
		assert.Equal(t, map[string]string{"_s_": "test-session"}, res.CState)
		assert.Empty(t, res.Broadcasts)
	})
}

func TestPerform(t *testing.T) {
	controller := NewTestController()
	client := mocks.RPCClient{}
	controller.client = &client

	t.Run("Success", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=secret;"}
		cstate := map[string]string{"_s_": "id=42"}

		client.On("Command", mock.Anything,
			&pb.CommandMessage{
				Command:               "message",
				ConnectionIdentifiers: "ids",
				Identifier:            "test_channel",
				Data:                  "hello",
				Env:                   &pb.Env{Url: url, Headers: headers, Cstate: cstate},
			}).Return(
			&pb.CommandResponse{
				Status:        pb.Status_SUCCESS,
				Streams:       []string{"chat_42"},
				StopStreams:   true,
				Env:           &pb.EnvResponse{Cstate: map[string]string{"_s_": "sentCount=1"}},
				Transmissions: []string{"message_sent"},
			}, nil)

		res, err := controller.Perform(
			"42",
			&common.SessionEnv{URL: url, Headers: &headers, ConnectionState: &cstate},
			"ids", "test_channel", "hello",
		)

		assert.Nil(t, err)
		assert.Equal(t, []string{"message_sent"}, res.Transmissions)
		assert.Equal(t, map[string]string{"_s_": "sentCount=1"}, res.CState)
		assert.True(t, res.StopAllStreams)
		assert.Equal(t, []string{"chat_42"}, res.Streams)
		assert.Nil(t, res.StoppedStreams)
		assert.Empty(t, res.Broadcasts)
	})

	t.Run("With stopped streams", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=secret;"}
		cstate := map[string]string{"_s_": "id=42"}

		client.On("Command", mock.Anything,
			&pb.CommandMessage{
				Command:               "message",
				ConnectionIdentifiers: "ids",
				Identifier:            "test_channel",
				Data:                  "stop_stream",
				Env:                   &pb.Env{Url: url, Headers: headers, Cstate: cstate},
			}).Return(
			&pb.CommandResponse{
				Status:         pb.Status_SUCCESS,
				StoppedStreams: []string{"chat_42"},
				StopStreams:    false,
				Env:            &pb.EnvResponse{Cstate: map[string]string{"_s_": "sentCount=1"}},
				Transmissions:  []string{"message_sent"},
			}, nil)

		res, err := controller.Perform(
			"42",
			&common.SessionEnv{URL: url, Headers: &headers, ConnectionState: &cstate},
			"ids", "test_channel", "stop_stream",
		)

		assert.Nil(t, err)
		assert.Equal(t, []string{"message_sent"}, res.Transmissions)
		assert.Equal(t, map[string]string{"_s_": "sentCount=1"}, res.CState)
		assert.False(t, res.StopAllStreams)
		assert.Equal(t, []string{"chat_42"}, res.StoppedStreams)
		assert.Nil(t, res.Streams)
		assert.Empty(t, res.Broadcasts)
	})

	t.Run("With channel state", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=secret;"}
		istate := map[string]string{"room": "room:1"}

		channels := make(map[string]map[string]string)
		channels["test_channel"] = istate

		client.On("Command", mock.Anything,
			&pb.CommandMessage{
				Command:               "message",
				ConnectionIdentifiers: "ids",
				Identifier:            "test_channel",
				Data:                  "channel_state",
				Env:                   &pb.Env{Url: url, Headers: headers, Istate: istate},
			}).Return(
			&pb.CommandResponse{
				Status:         pb.Status_SUCCESS,
				StoppedStreams: []string{"chat_42"},
				StopStreams:    false,
				Env:            &pb.EnvResponse{Istate: map[string]string{"count": "1"}},
				Transmissions:  []string{"message_sent"},
			}, nil)

		res, err := controller.Perform(
			"42",
			&common.SessionEnv{URL: url, Headers: &headers, ChannelStates: &channels},
			"ids", "test_channel", "channel_state",
		)

		assert.Nil(t, err)
		assert.Equal(t, []string{"message_sent"}, res.Transmissions)
		assert.Equal(t, map[string]string{"count": "1"}, res.IState)
		assert.False(t, res.StopAllStreams)
		assert.Equal(t, []string{"chat_42"}, res.StoppedStreams)
		assert.Nil(t, res.Streams)
		assert.Empty(t, res.Broadcasts)
	})
}

func TestSubscribe(t *testing.T) {
	controller := NewTestController()
	client := mocks.RPCClient{}
	controller.client = &client

	t.Run("Success", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=secret;"}
		cstate := map[string]string{"_s_": "id=42"}

		client.On("Command", mock.Anything,
			&pb.CommandMessage{
				Command:               "subscribe",
				ConnectionIdentifiers: "ids",
				Identifier:            "test_channel",
				Env:                   &pb.Env{Url: url, Headers: headers, Cstate: cstate},
			}).Return(
			&pb.CommandResponse{
				Status:        pb.Status_SUCCESS,
				Streams:       []string{"chat_42"},
				Env:           &pb.EnvResponse{Cstate: map[string]string{"_s_": "sentCount=1"}},
				Transmissions: []string{"confirmed"},
			}, nil)

		res, err := controller.Subscribe(
			"42",
			&common.SessionEnv{URL: url, Headers: &headers, ConnectionState: &cstate},
			"ids", "test_channel",
		)

		assert.Nil(t, err)
		assert.Equal(t, []string{"confirmed"}, res.Transmissions)
		assert.Equal(t, map[string]string{"_s_": "sentCount=1"}, res.CState)
		assert.False(t, res.StopAllStreams)
		assert.Equal(t, []string{"chat_42"}, res.Streams)
		assert.Nil(t, res.StoppedStreams)
		assert.Empty(t, res.Broadcasts)
	})

	t.Run("Failure", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=secret;"}
		cstate := map[string]string{"_s_": "id=42"}

		client.On("Command", mock.Anything,
			&pb.CommandMessage{
				Command:               "subscribe",
				ConnectionIdentifiers: "ids",
				Identifier:            "fail_channel",
				Env:                   &pb.Env{Url: url, Headers: headers, Cstate: cstate},
			}).Return(
			&pb.CommandResponse{
				Status:        pb.Status_FAILURE,
				ErrorMsg:      "Unauthorized",
				Disconnect:    true,
				Transmissions: []string{"rejected"},
			}, nil)

		res, err := controller.Subscribe(
			"42",
			&common.SessionEnv{URL: url, Headers: &headers, ConnectionState: &cstate},
			"ids", "fail_channel",
		)

		assert.NotNil(t, err)
		assert.Equal(t, []string{"rejected"}, res.Transmissions)
		assert.True(t, res.Disconnect)
		assert.Nil(t, res.StoppedStreams)
		assert.Empty(t, res.Broadcasts)
	})
}

func TestDisconnect(t *testing.T) {
	controller := NewTestController()
	client := mocks.RPCClient{}
	controller.client = &client

	t.Run("Success", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=secret;"}
		cstate := map[string]string{"_s_": "id=42"}
		istate := map[string]string{"test_channel": "{\"room\":\"room:1\"}"}

		channels := make(map[string]map[string]string)
		channels["test_channel"] = map[string]string{"room": "room:1"}

		client.On("Disconnect", mock.Anything,
			&pb.DisconnectRequest{
				Identifiers:   "ids",
				Subscriptions: []string{"chat_42"},
				Path:          url,
				Headers:       headers,
				Env:           &pb.Env{Url: url, Headers: headers, Cstate: cstate, Istate: istate},
			}).Return(
			&pb.DisconnectResponse{
				Status: pb.Status_SUCCESS,
			}, nil)

		err := controller.Disconnect(
			"42",
			&common.SessionEnv{URL: url, Headers: &headers, ConnectionState: &cstate, ChannelStates: &channels},
			"ids",
			[]string{"chat_42"},
		)
		assert.Nil(t, err)
	})
}
