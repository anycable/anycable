package pusher

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/node"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// From https://pusher.com/docs/channels/library_auth_reference/auth-signatures/
const (
	app_id             = "278d425bdf160c739803"
	key                = "7ad3773142a6692b25b8"
	stream             = "private-foobar"
	signature          = "278d425bdf160c739803:58df8b0c36d6982b82c3ecf6b4662e34fe8c25bba48f5369f135bf843651c3a4"
	presence_stream    = "presence-foobar"
	presence_signature = "278d425bdf160c739803:31935e7d86dba64c2a90aed31fdc61869f9b22ba9d8863bba239c03ca481bc80"
	presence_data      = "{\"user_id\":10,\"user_info\":{\"name\":\"Mr. Channels\"}}"
)

type MockExecutor struct {
	ctrl *Controller
}

func NewMockExecutor(ctrl *Controller) *MockExecutor {
	return &MockExecutor{ctrl: ctrl}
}

func (e *MockExecutor) HandleCommand(s *node.Session, msg *common.Message) (err error) {
	var res *common.CommandResult

	if msg.Command == "subscribe" {
		res, err = e.ctrl.Subscribe(s.GetID(), s.GetEnv(), s.GetIdentifiers(), msg.Identifier)
	}

	if res != nil {
		for _, msg := range res.Transmissions {
			s.SendJSONTransmission(msg)
		}
	}

	if res == nil {
		return errors.New("not implemeented")
	}

	return
}

func (e *MockExecutor) Disconnect(s *node.Session) error {
	return nil
}

func TestHandleCommand(t *testing.T) {
	conf := NewConfig()
	conf.AppKey = app_id
	conf.Secret = key
	app := NewMockExecutor(NewController(&conf, slog.Default()))
	n := NewMockNode()
	verifier := NewVerifier(app_id, key)
	executor := NewExecutor(app, verifier)

	t.Run("Subscribe (public)", func(t *testing.T) {
		conn := mocks.NewMockConnection()
		session := buildSession(conn, n, executor)
		defer session.Disconnect("test", 0)

		msg := common.Message{Command: "subscribe", Identifier: fmt.Sprintf(`{"channel":"$pusher","stream":"%s"}`, "all-chat")}

		err := executor.HandleCommand(
			session, &msg,
		)
		require.NoError(t, err)

		res, err := conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `{"event":"pusher_internal:subscription_succeeded","data":{},"channel":"all-chat"}`, string(res))
	})

	t.Run("Subscribe (private)", func(t *testing.T) {
		conn := mocks.NewMockConnection()
		session := buildSession(conn, n, executor)
		defer session.Disconnect("test", 0)

		msg := common.Message{Command: "subscribe", Identifier: fmt.Sprintf(`{"channel":"$pusher","stream":"%s"}`, stream), Data: &PusherSubscriptionData{Auth: signature, Channel: stream}}

		err := executor.HandleCommand(
			session, &msg,
		)
		require.NoError(t, err)

		res, err := conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `{"event":"pusher_internal:subscription_succeeded","data":{},"channel":"private-foobar"}`, string(res))
	})

	t.Run("Subscribe (private + failure bad signature)", func(t *testing.T) {
		conn := mocks.NewMockConnection()
		session := buildSession(conn, n, executor)
		defer session.Disconnect("test", 0)

		msg := common.Message{Command: "subscribe", Identifier: fmt.Sprintf(`{"channel":"$pusher","stream":"%s"}`, stream), Data: &PusherSubscriptionData{Auth: "bla-bla"}}

		err := executor.HandleCommand(
			session, &msg,
		)
		require.NoError(t, err)

		res, err := conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `{"event":"pusher:error","data":{"message":"rejected","code":4009}}`, string(res))
	})

	t.Run("Subscribe (presence)", func(t *testing.T) {
		conn := mocks.NewMockConnection()
		session := buildSession(conn, n, executor)
		defer session.Disconnect("test", 0)

		msg := common.Message{Command: "subscribe", Identifier: fmt.Sprintf(`{"channel":"$pusher","stream":"%s"}`, presence_stream), Data: &PusherSubscriptionData{Auth: presence_signature, Channel: presence_stream, ChannelData: presence_data}}

		err := executor.HandleCommand(
			session, &msg,
		)
		require.NoError(t, err)

		res, err := conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `{"event":"pusher_internal:subscription_succeeded","data":{},"channel":"presence-foobar"}`, string(res))
	})
}

func buildSession(conn node.Connection, n *node.Node, executor node.Executor) *node.Session {
	s := node.NewSession(n, conn, "ws://anycable.io/pusher/app-id", nil, "1234.1234", node.WithEncoder(&Encoder{}), node.WithExecutor(executor))
	s.Connected = true
	s.Log = slog.With("context", "test")
	return s
}

// NewMockNode build new node with mock controller
func NewMockNode() *node.Node {
	controller := mocks.NewMockController()
	config := node.NewConfig()
	config.HubGopoolSize = 2
	node := node.NewNode(&config, node.WithController(&controller), node.WithInstrumenter(metrics.NewMetrics(nil, 10, slog.Default())))
	return node
}
