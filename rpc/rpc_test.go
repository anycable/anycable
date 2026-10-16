package rpc

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	pb "github.com/anycable/anycable-go/protos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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
	metrics := metrics.NewMetrics(nil, 0, slog.Default())
	controller, _ := NewController(metrics, &config, slog.Default())
	barrier, _ := NewFixedSizeBarrier(5)
	controller.barrier = barrier
	controller.clientState = MockState{true, false}
	return controller
}

func TestAuthenticate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		controller := NewTestController()
		client := mocks.RPCClient{}
		controller.client = &client
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

		res, err := controller.Authenticate(context.Background(), "42", &common.SessionEnv{URL: url, Headers: &headers})
		assert.Nil(t, err)
		assert.Equal(t, []string{"welcome"}, res.Transmissions)
		assert.Equal(t, "user=john", res.Identifier)
		assert.Equal(t, map[string]string{"_s_": "test-session"}, res.CState)
		assert.Empty(t, res.Broadcasts)
	})

	t.Run("Failure", func(t *testing.T) {
		controller := NewTestController()
		client := mocks.RPCClient{}
		controller.client = &client
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

		res, err := controller.Authenticate(context.Background(), "42", &common.SessionEnv{URL: url, Headers: &headers})
		assert.Nil(t, err)
		assert.Equal(t, []string{"unauthorized"}, res.Transmissions)
		assert.Equal(t, "", res.Identifier)
		assert.Equal(t, map[string]string{"_s_": "test-session"}, res.CState)
		assert.Empty(t, res.Broadcasts)
	})

	t.Run("Error", func(t *testing.T) {
		controller := NewTestController()
		client := mocks.RPCClient{}
		controller.client = &client
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

		res, err := controller.Authenticate(context.Background(), "42", &common.SessionEnv{URL: url, Headers: &headers})
		assert.NotNil(t, err)
		assert.Error(t, err, "Exception")
		assert.Nil(t, res.Transmissions)
		assert.Equal(t, "", res.Identifier)
		assert.Nil(t, res.CState)
		assert.Empty(t, res.Broadcasts)
	})

	t.Run("Canceled before RPC", func(t *testing.T) {
		controller := NewTestController()
		client := mocks.RPCClient{}
		controller.client = &client
		url := "/cancel-cables"
		headers := make(map[string]string)

		canceledCtx, cancel := context.WithCancel(context.Background())
		// cancel right away
		cancel()

		expectedReq := pb.ConnectionRequest{
			Env: &pb.Env{Url: url, Headers: headers},
		}
		client.On("Connect", mock.Anything,
			&expectedReq).Return(
			nil, canceledCtx.Err())

		res, err := controller.Authenticate(canceledCtx, "42", &common.SessionEnv{URL: url, Headers: &headers})
		assert.Nil(t, err)
		assert.Nil(t, res)

		client.AssertNotCalled(t, "Connect", mock.Anything, &expectedReq)
	})

	t.Run("Canceled during RPC", func(t *testing.T) {
		controller := NewTestController()
		client := mocks.RPCClient{}
		controller.client = &client
		url := "/cancel-cables-2"
		headers := make(map[string]string)

		expectedReq := pb.ConnectionRequest{
			Env: &pb.Env{Url: url, Headers: headers},
		}

		canceledCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		client.On("Connect", mock.Anything,
			&expectedReq).Return(
			nil,
			func(ctx context.Context, req *pb.ConnectionRequest, opts ...grpc.CallOption) error {
				cancel()
				return canceledCtx.Err()
			})

		res, err := controller.Authenticate(canceledCtx, "42", &common.SessionEnv{URL: url, Headers: &headers})
		assert.Nil(t, err)
		assert.Nil(t, res)

		client.AssertCalled(t, "Connect", mock.Anything, &expectedReq)
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
			context.Background(),
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
			context.Background(),
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
			context.Background(),
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
			context.Background(),
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
			context.Background(),
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

	t.Run("With presence", func(t *testing.T) {
		url := "/cable-present"
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
				Env:            &pb.EnvResponse{Istate: map[string]string{common.PRESENCE_STREAM_STATE: "presence_42"}},
				Transmissions:  []string{"message_sent"},
				Presence: &pb.PresenceResponse{
					Type: "leave",
					Id:   "2024",
				},
			}, nil)

		res, err := controller.Perform(
			context.Background(),
			"42",
			&common.SessionEnv{URL: url, Headers: &headers, ChannelStates: &channels},
			"ids", "test_channel", "channel_state",
		)

		assert.Nil(t, err)
		assert.Equal(t, []string{"message_sent"}, res.Transmissions)
		assert.Equal(t, map[string]string{"$p": "presence_42"}, res.IState)
		assert.False(t, res.StopAllStreams)
		assert.Equal(t, []string{"chat_42"}, res.StoppedStreams)
		assert.Nil(t, res.Streams)
		assert.Empty(t, res.Broadcasts)

		require.NotNil(t, res.Presence)
		assert.Equal(t, common.PresenceLeaveType, res.Presence.Type)
		assert.Equal(t, "2024", res.Presence.ID)
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
			context.Background(),
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
			context.Background(),
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
			context.Background(),
			"42",
			&common.SessionEnv{URL: url, Headers: &headers, ConnectionState: &cstate},
			"ids", "error_channel",
		)

		assert.NotNil(t, err)
		assert.Equal(t, common.ERROR, res.Status)
	})

	t.Run("With presence", func(t *testing.T) {
		url := "/cable-present"
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
				Presence: &pb.PresenceResponse{
					Type: "join",
					Id:   "2024",
					Info: `{"name":"Torcha"}`,
				},
			}, nil)

		res, err := controller.Subscribe(
			context.Background(),
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

		require.NotNil(t, res.Presence)
		assert.Equal(t, common.PresenceJoinType, res.Presence.Type)
		assert.Equal(t, "2024", res.Presence.ID)
		assert.Equal(t, "Torcha", (res.Presence.Info.(map[string]interface{}))["name"])
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
			context.Background(),
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

	controller, err := NewController(metrics.NewMetrics(nil, 0, slog.Default()), &config, slog.Default())
	require.NoError(t, err)
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

		res, err := controller.Authenticate(context.Background(), "42", &common.SessionEnv{URL: url, Headers: &headers})
		require.Nilf(t, err, "Error: %v", err)
		assert.Equal(t, []string{"welcome"}, res.Transmissions)
		assert.Equal(t, "user=john", res.Identifier)
		assert.Equal(t, map[string]string{"_s_": "test-session"}, res.CState)
		assert.Empty(t, res.Broadcasts)

		call := service.Calls[0]
		requestCtx, ok := call.Arguments[0].(context.Context)

		require.True(t, ok)

		md, ok := metadata.FromIncomingContext(requestCtx)
		require.True(t, ok)

		assert.Equal(t, []string{"42"}, md.Get("sid"))
	})
}

func TestWaitTimeout(t *testing.T) {
	newBusyController := func(t *testing.T) (*Controller, *mocks.RPCClient) {
		controller := NewTestController()
		controller.waitTimeout = 50 * time.Millisecond
		barrier, _ := NewFixedSizeBarrier(1)
		controller.barrier = barrier
		client := &mocks.RPCClient{}
		controller.client = client

		// Take the only slot
		require.NoError(t, barrier.Acquire(context.Background()))

		return controller, client
	}

	counter := func(c *Controller, name string) uint64 {
		return c.metrics.(*metrics.Metrics).Counter(name).Value()
	}

	t.Run("Defaults to the command timeout", func(t *testing.T) {
		config := NewConfig()
		config.CommandTimeout = 1234
		controller, _ := NewController(metrics.NewMetrics(nil, 0, slog.Default()), &config, slog.Default())

		assert.Equal(t, 1234*time.Millisecond, controller.waitTimeout)

		config.WaitTimeout = 500
		controller, _ = NewController(metrics.NewMetrics(nil, 0, slog.Default()), &config, slog.Default())

		assert.Equal(t, 500*time.Millisecond, controller.waitTimeout)
	})

	t.Run("Zero wait timeout means no limit", func(t *testing.T) {
		controller, client := newBusyController(t)
		controller.waitTimeout = 0

		client.On("Command", mock.Anything, mock.Anything).Return(
			&pb.CommandResponse{Status: pb.Status_SUCCESS}, nil)

		time.AfterFunc(100*time.Millisecond, controller.barrier.Release)

		headers := map[string]string{}
		_, err := controller.Subscribe(context.Background(), "42", &common.SessionEnv{Headers: &headers}, "ids", "test_channel")

		assert.Nil(t, err)
		client.AssertCalled(t, "Command", mock.Anything, mock.Anything)
	})

	t.Run("Subscribe times out waiting for a slot", func(t *testing.T) {
		controller, client := newBusyController(t)

		res, err := controller.Subscribe(context.Background(), "42", &common.SessionEnv{}, "ids", "test_channel")

		assert.Nil(t, res)
		assert.ErrorIs(t, err, ErrWaitTimeout)
		client.AssertNotCalled(t, "Command", mock.Anything, mock.Anything)
		assert.Equal(t, uint64(1), counter(controller, metricsRPCWaitTimeouts))
		assert.Equal(t, uint64(0), counter(controller, metricsRPCCanceled))
		assert.Equal(t, uint64(0), controller.metrics.(*metrics.Metrics).Gauge(metricsRPCPending).Value())
	})

	t.Run("Authenticate times out waiting for a slot", func(t *testing.T) {
		controller, client := newBusyController(t)

		res, err := controller.Authenticate(context.Background(), "42", &common.SessionEnv{})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, ErrWaitTimeout)
		client.AssertNotCalled(t, "Connect", mock.Anything, mock.Anything)
	})

	t.Run("Session closes while waiting for a slot", func(t *testing.T) {
		controller, client := newBusyController(t)
		controller.waitTimeout = 0

		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(10*time.Millisecond, cancel)

		res, err := controller.Perform(ctx, "42", &common.SessionEnv{}, "ids", "test_channel", "data")

		assert.Nil(t, res)
		assert.Nil(t, err)
		client.AssertNotCalled(t, "Command", mock.Anything, mock.Anything)
		assert.Equal(t, uint64(1), counter(controller, metricsRPCCanceled))
		assert.Equal(t, uint64(0), counter(controller, metricsRPCWaitTimeouts))
	})

	t.Run("Disconnect waits longer than the wait timeout", func(t *testing.T) {
		controller, client := newBusyController(t)

		client.On("Disconnect", mock.Anything, mock.Anything).Return(
			&pb.DisconnectResponse{Status: pb.Status_SUCCESS}, nil)

		time.AfterFunc(2*controller.waitTimeout, controller.barrier.Release)

		headers := map[string]string{}
		err := controller.Disconnect(context.Background(), "42", &common.SessionEnv{Headers: &headers}, "ids", []string{})

		assert.Nil(t, err)
		client.AssertCalled(t, "Disconnect", mock.Anything, mock.Anything)
		assert.Equal(t, uint64(0), counter(controller, metricsRPCWaitTimeouts))
	})
}
