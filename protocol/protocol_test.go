package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/anycable/anycable-go/common"

	pb "github.com/anycable/anycable-go/protos"
)

func buildSessionEnv() *common.SessionEnv {
	url := "/cable-test"
	headers := map[string]string{"cookie": "token=secret;"}

	return &common.SessionEnv{URL: url, Headers: &headers}
}

func TestNewConnectMessage(t *testing.T) {
	env := buildSessionEnv()

	msg := NewConnectMessage(env)

	assert.NotNil(t, msg.Env)
	assert.Equal(t, "/cable-test", msg.Env.Url)
	assert.Equal(t, map[string]string{"cookie": "token=secret;"}, msg.Env.Headers)
}

func TestNewCommandMessage(t *testing.T) {
	t.Run("Base", func(t *testing.T) {
		env := buildSessionEnv()

		msg := NewCommandMessage(env, "subscribe", "test_channel", "user=john", "")

		assert.Equal(t, "subscribe", msg.Command)
		assert.Equal(t, "test_channel", msg.Identifier)
		assert.Equal(t, "user=john", msg.ConnectionIdentifiers)
		assert.NotNil(t, msg.Env)

		assert.NotNil(t, "/cable-test", msg.Env.Url)
		assert.Equal(t, map[string]string{"cookie": "token=secret;"}, msg.Env.Headers)
	})

	t.Run("With connection, channel state and data", func(t *testing.T) {
		env := buildSessionEnv()
		cstate := map[string]string{"_s_": "id=42"}
		env.ConnectionState = &cstate

		istate := map[string]string{"room": "room:1"}
		channels := make(map[string]map[string]string)
		channels["test_channel"] = istate

		env.ChannelStates = &channels

		msg := NewCommandMessage(env, "subscribe", "test_channel", "user=john", "action")

		assert.Equal(t, "subscribe", msg.Command)
		assert.Equal(t, "test_channel", msg.Identifier)
		assert.Equal(t, "user=john", msg.ConnectionIdentifiers)
		assert.Equal(t, "action", msg.Data)
		assert.NotNil(t, msg.Env)

		assert.Equal(t, cstate, msg.Env.Cstate)
		assert.Equal(t, istate, msg.Env.Istate)
	})
}

func TestNewDisconnectRequest(t *testing.T) {
	env := buildSessionEnv()

	cstate := map[string]string{"_s_": "id=42"}
	istate := map[string]string{"test_channel": "{\"room\":\"room:1\"}"}

	channels := make(map[string]map[string]string)
	channels["test_channel"] = map[string]string{"room": "room:1"}

	env.ConnectionState = &cstate
	env.ChannelStates = &channels

	msg := NewDisconnectMessage(env, "user=john", []string{"chat_42"})

	assert.Equal(t, "user=john", msg.Identifiers)
	assert.Equal(t, []string{"chat_42"}, msg.Subscriptions)
	assert.Equal(t, cstate, msg.Env.Cstate)
	assert.Equal(t, istate, msg.Env.Istate)
}

func TestParseConnectResponse(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		res := pb.ConnectionResponse{
			Identifiers:   "user=john",
			Transmissions: []string{"welcome"},
			Status:        pb.Status_SUCCESS,
			Env:           &pb.EnvResponse{Cstate: map[string]string{"_s_": "test-session"}},
		}

		result, err := ParseConnectResponse(&res)

		assert.Nil(t, err)
		assert.Equal(t, "user=john", result.Identifier)
		assert.Equal(t, []string{"welcome"}, result.Transmissions)
		assert.Equal(t, map[string]string{"_s_": "test-session"}, result.CState)
	})
	t.Run("Failure", func(t *testing.T) {
		res := pb.ConnectionResponse{
			Transmissions: []string{"unauthorized"},
			Status:        pb.Status_FAILURE,
			Env:           &pb.EnvResponse{Cstate: map[string]string{"_s_": "test-session"}},
			ErrorMsg:      "Authentication failed",
		}

		_, err := ParseConnectResponse(&res)

		assert.NotNil(t, err)
	})
}

func TestParseCommandResponse(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		res := pb.CommandResponse{
			Status:         pb.Status_SUCCESS,
			Streams:        []string{"chat_42"},
			StoppedStreams: []string{"chat_41"},
			StopStreams:    true,
			Transmissions:  []string{"message_sent"},
		}

		result, err := ParseCommandResponse(&res)

		assert.Nil(t, err)
		assert.Equal(t, []string{"chat_42"}, result.Streams)
		assert.Equal(t, []string{"message_sent"}, result.Transmissions)
		assert.Equal(t, true, result.StopAllStreams)
		assert.Equal(t, []string{"chat_41"}, result.StoppedStreams)
	})

	t.Run("Success with connection and channel state", func(t *testing.T) {
		res := pb.CommandResponse{
			Status:        pb.Status_SUCCESS,
			Streams:       []string{"chat_42"},
			Env:           &pb.EnvResponse{Cstate: map[string]string{"_s_": "sentCount=1"}, Istate: map[string]string{"count": "1"}},
			Transmissions: []string{"message_sent"},
		}

		result, err := ParseCommandResponse(&res)

		assert.Nil(t, err)
		assert.Equal(t, []string{"chat_42"}, result.Streams)
		assert.Equal(t, []string{"message_sent"}, result.Transmissions)
		assert.Equal(t, map[string]string{"_s_": "sentCount=1"}, result.CState)
		assert.Equal(t, map[string]string{"count": "1"}, result.IState)
	})

	t.Run("Failure", func(t *testing.T) {
		res := pb.CommandResponse{
			Status:   pb.Status_FAILURE,
			ErrorMsg: "Unknown command",
		}

		_, err := ParseCommandResponse(&res)

		assert.NotNil(t, err)
	})
}

func TestParseDisconnectResponse(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		res := pb.DisconnectResponse{
			Status: pb.Status_SUCCESS,
		}

		err := ParseDisconnectResponse(&res)

		assert.Nil(t, err)
	})
	t.Run("Failure", func(t *testing.T) {
		res := pb.DisconnectResponse{
			Status: pb.Status_FAILURE,
		}

		err := ParseDisconnectResponse(&res)

		assert.NotNil(t, err)
	})
}
