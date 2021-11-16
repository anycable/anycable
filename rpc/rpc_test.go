package rpc

import (
	"errors"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	pb "github.com/anycable/anycable-go/protos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockState struct {
	ready  bool
	closed bool
}

func (st MockState) Ready() error {
	if st.ready {
		return nil
	}

	return errors.New("not ready")
}

func (st MockState) Close() {
}

func (st MockState) SupportsActiveConns() bool {
	return false
}

func (st MockState) ActiveConns() int {
	return 0
}

func NewTestController() *Controller {
	config := NewConfig()
	metrics := metrics.NewMetrics(nil, 0)
	controller := NewController(metrics, &config)
	controller.barrier = NewFixedSizeBarrier(5)
	controller.clientState = MockState{true, false}
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
				Env: &pb.Env{Url: url, Headers: headers},
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
				Env: &pb.Env{Url: url, Headers: headers},
			}).Return(
			&pb.ConnectionResponse{
				Transmissions: []string{"unauthorized"},
				Status:        pb.Status_FAILURE,
				Env:           &pb.EnvResponse{Cstate: map[string]string{"_s_": "test-session"}},
				ErrorMsg:      "Authentication failed",
			}, nil)

		res, err := controller.Authenticate("42", &common.SessionEnv{URL: url, Headers: &headers})
		assert.Nil(t, err)
		assert.Equal(t, []string{"unauthorized"}, res.Transmissions)
		assert.Equal(t, "", res.Identifier)
		assert.Equal(t, map[string]string{"_s_": "test-session"}, res.CState)
		assert.Empty(t, res.Broadcasts)
	})

	t.Run("Error", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=exceptional;"}

		client.On("Connect", mock.Anything,
			&pb.ConnectionRequest{
				Env: &pb.Env{Url: url, Headers: headers},
			}).Return(
			&pb.ConnectionResponse{
				Status:   pb.Status_ERROR,
				ErrorMsg: "Exception",
			}, nil)

		res, err := controller.Authenticate("42", &common.SessionEnv{URL: url, Headers: &headers})
		assert.NotNil(t, err)
		assert.Error(t, err, "Exception")
		assert.Nil(t, res.Transmissions)
		assert.Equal(t, "", res.Identifier)
		assert.Nil(t, res.CState)
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

	t.Run("Failure", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=invalid;"}
		cstate := map[string]string{"_s_": "id=42"}

		client.On("Command", mock.Anything,
			&pb.CommandMessage{
				Command:               "message",
				ConnectionIdentifiers: "ids",
				Identifier:            "test_channel",
				Data:                  "fail",
				Env:                   &pb.Env{Url: url, Headers: headers, Cstate: cstate},
			}).Return(
			&pb.CommandResponse{
				Status:        pb.Status_FAILURE,
				Streams:       []string{"chat_42"},
				StopStreams:   true,
				Env:           &pb.EnvResponse{Cstate: map[string]string{"_s_": "sentCount=1"}},
				Transmissions: []string{"message_sent"},
				ErrorMsg:      "Forbidden",
			}, nil)

		res, err := controller.Perform(
			"42",
			&common.SessionEnv{URL: url, Headers: &headers, ConnectionState: &cstate},
			"ids", "test_channel", "fail",
		)

		assert.Nil(t, err)
		assert.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{"message_sent"}, res.Transmissions)
		assert.Equal(t, map[string]string{"_s_": "sentCount=1"}, res.CState)
		assert.True(t, res.StopAllStreams)
		assert.Equal(t, []string{"chat_42"}, res.Streams)
		assert.Nil(t, res.StoppedStreams)
		assert.Empty(t, res.Broadcasts)
	})

	t.Run("Error", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=invalid;"}
		cstate := map[string]string{"_s_": "id=42"}

		client.On("Command", mock.Anything,
			&pb.CommandMessage{
				Command:               "message",
				ConnectionIdentifiers: "ids",
				Identifier:            "test_channel",
				Data:                  "exception",
				Env:                   &pb.Env{Url: url, Headers: headers, Cstate: cstate},
			}).Return(
			&pb.CommandResponse{
				Status:      pb.Status_ERROR,
				StopStreams: true,
				ErrorMsg:    "Exception",
			}, nil)

		res, err := controller.Perform(
			"42",
			&common.SessionEnv{URL: url, Headers: &headers, ConnectionState: &cstate},
			"ids", "test_channel", "exception",
		)

		assert.NotNil(t, err)
		assert.Equal(t, common.ERROR, res.Status)
		assert.Error(t, err, "Exception")
		assert.Nil(t, res.Transmissions)
		assert.True(t, res.StopAllStreams)
		assert.Nil(t, res.Streams)
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

		assert.Nil(t, err)
		assert.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{"rejected"}, res.Transmissions)
		assert.True(t, res.Disconnect)
		assert.Nil(t, res.StoppedStreams)
		assert.Empty(t, res.Broadcasts)
	})

	t.Run("Error", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=secret;"}
		cstate := map[string]string{"_s_": "id=42"}

		client.On("Command", mock.Anything,
			&pb.CommandMessage{
				Command:               "subscribe",
				ConnectionIdentifiers: "ids",
				Identifier:            "error_channel",
				Env:                   &pb.Env{Url: url, Headers: headers, Cstate: cstate},
			}).Return(
			&pb.CommandResponse{
				Status:   pb.Status_ERROR,
				ErrorMsg: "Exception",
			}, nil)

		res, err := controller.Subscribe(
			"42",
			&common.SessionEnv{URL: url, Headers: &headers, ConnectionState: &cstate},
			"ids", "error_channel",
		)

		assert.NotNil(t, err)
		assert.Equal(t, common.ERROR, res.Status)
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

func TestCustomDialFun(t *testing.T) {
	config := NewConfig()

	service := mocks.RPCServer{}
	stateHandler := MockState{true, false}

	config.DialFun = NewInprocessServiceDialer(&service, stateHandler)

	controller := NewController(metrics.NewMetrics(nil, 0), &config)
	require.NoError(t, controller.Start())

	t.Run("Connect", func(t *testing.T) {
		url := "/cable-test"
		headers := map[string]string{"cookie": "token=secret;"}

		service.On("Connect", mock.Anything,
			&pb.ConnectionRequest{
				Env: &pb.Env{Url: url, Headers: headers},
			}).Return(
			&pb.ConnectionResponse{
				Identifiers:   "user=john",
				Transmissions: []string{"welcome"},
				Status:        pb.Status_SUCCESS,
				Env:           &pb.EnvResponse{Cstate: map[string]string{"_s_": "test-session"}},
			}, nil)

		res, err := controller.Authenticate("42", &common.SessionEnv{URL: url, Headers: &headers})
		require.Nil(t, err)
		assert.Equal(t, []string{"welcome"}, res.Transmissions)
		assert.Equal(t, "user=john", res.Identifier)
		assert.Equal(t, map[string]string{"_s_": "test-session"}, res.CState)
		assert.Empty(t, res.Broadcasts)
	})
}
