package node

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// A test to verify the restore flow.
//
// SETUP:
// - A session is created and suscribed to some channels/streams.
// - A few broadcasts/commands are made to ensure that subscription works and
// the session's state is modified
// - Session disconnects.
//
// EXECUTE:
// - A new session is initiated with the sid of the previous one.
//
// TEST 1 — hub subscriptions:
// - Made some broadcasts to the old session streams.
// - A new session MUST receive the messages.
//
// TEST 2 — connection/channel state:
// - Execute a command which echoes back the states.
// - Verifies the received messages.
//
// TEST 3 — expired cache:
// - Wait for cache to expire
// - Make sure it is not restored (uses controller.Authenticate)
func TestIntegrationRestore_Memory(t *testing.T) {
	node, controller := setupIntegrationNode()
	go node.Start()                           // nolint:errcheck
	defer node.Shutdown(context.Background()) // nolint:errcheck

	sharedIntegrationRestore(t, node, controller)
}

func sharedIntegrationRestore(t *testing.T, node *Node, controller *mocks.Controller) {
	sid := "s18"
	ids := "user:jack"

	prev_session := NewMockSessionWithEnv(sid, node, "ws://test.anycable.io/cable", nil)

	controller.
		On("Authenticate", sid, prev_session.env).
		Return(&common.ConnectResult{
			Identifier:    ids,
			Status:        common.SUCCESS,
			Transmissions: []string{`{"type":"welcome"}`},
			CState:        map[string]string{"city": "Napoli"},
		}, nil)

	_, err := node.Authenticate(prev_session)
	require.NoError(t, err)

	requireReceive(
		t,
		prev_session,
		`{"type":"welcome"}`,
	)

	// Subscribe the channels
	controller.
		On("Subscribe", sid, prev_session.env, ids, "chat_1").
		Return(&common.CommandResult{
			Status:        common.SUCCESS,
			Transmissions: []string{`{"type":"confirm","identifier":"chat_1"}`},
			Streams:       []string{"presence_1", "messages_1"},
		}, nil)
	controller.
		On("Subscribe", sid, prev_session.env, ids, "user_jack").
		Return(&common.CommandResult{
			Status:        common.SUCCESS,
			Transmissions: []string{`{"type":"confirm","identifier":"user_jack"}`},
			Streams:       []string{"u_jack"},
			IState:        map[string]string{"locale": "it"},
		}, nil)

	_, err = node.Subscribe(prev_session, &common.Message{Identifier: "chat_1", Command: "subscribe"})
	require.NoError(t, err)

	requireReceive(
		t,
		prev_session,
		`{"type":"confirm","identifier":"chat_1"}`,
	)

	_, err = node.Subscribe(prev_session, &common.Message{Identifier: "user_jack", Command: "subscribe"})
	require.NoError(t, err)

	requireReceive(
		t,
		prev_session,
		`{"type":"confirm","identifier":"user_jack"}`,
	)

	node.HandleBroadcast([]byte(`{"stream": "messages_1", "data": "Alice: Hey!"}`))
	requireReceive(t, prev_session, `{"identifier":"chat_1","message":"Alice: Hey!","stream_id":"messages_1","epoch":"2022","offset":1}`)

	node.HandleBroadcast([]byte(`{"stream": "u_jack", "data": "New message from Alice"}`))
	requireReceive(t, prev_session, `{"identifier":"user_jack","message":"New message from Alice","stream_id":"u_jack","epoch":"2022","offset":1}`)

	prev_session.Disconnect("normal", ws.CloseNormalClosure)

	session := NewMockSessionWithEnv("s21", node, fmt.Sprintf("ws://test.anycable.io/cable?sid=%s", sid), nil)

	_, err = node.Authenticate(session)
	require.NoError(t, err)

	welcomeMsg, err := session.conn.Read()
	require.NoError(t, err)

	var welcome map[string]interface{}
	err = json.Unmarshal(welcomeMsg, &welcome)
	require.NoError(t, err)

	require.Equal(t, "welcome", welcome["type"])
	require.Equal(t, "s21", welcome["sid"])
	require.Equal(t, true, welcome["restored"])
	require.Contains(t, welcome["restored_ids"], "chat_1")
	require.Contains(t, welcome["restored_ids"], "user_jack")

	t.Run("Restore hub subscriptions", func(t *testing.T) {
		node.HandleBroadcast([]byte(`{"stream": "messages_1", "data": "Lorenzo: Ciao"}`))
		requireReceive(t, session, `{"identifier":"chat_1","message":"Lorenzo: Ciao","stream_id":"messages_1","epoch":"2022","offset":2}`)

		node.HandleBroadcast([]byte(`{"stream": "presence_1", "data": "@lorenzo:join"}`))
		requireReceive(t, session, `{"identifier":"chat_1","message":"@lorenzo:join","stream_id":"presence_1","epoch":"2022","offset":1}`)

		node.HandleBroadcast([]byte(`{"stream": "u_jack", "data": "1:1"}`))
		requireReceive(t, session, `{"identifier":"user_jack","message":"1:1","stream_id":"u_jack","epoch":"2022","offset":2}`)
	})

	t.Run("Restore session connection and channels state", func(t *testing.T) {
		controller.
			On("Perform", "s21", mock.Anything, ids, "user_jack", "echo").
			Return(func(sid string, env *common.SessionEnv, ids string, identifier string, data string) *common.CommandResult {
				res := &common.CommandResult{Status: common.SUCCESS}
				res.Transmissions = []string{
					fmt.Sprintf("city:%s", env.GetConnectionStateField("city")),
					fmt.Sprintf("locale:%s", env.GetChannelStateField("user_jack", "locale")),
				}

				return res
			}, nil)

		_, perr := node.Perform(session, &common.Message{Identifier: "user_jack", Data: "echo", Command: "message"})
		require.NoError(t, perr)

		requireReceive(t, session, "city:Napoli")
		requireReceive(t, session, "locale:it")
	})

	t.Run("Not restored when cache expired", func(t *testing.T) {
		controller.
			On("Authenticate", "s42", mock.Anything).
			Return(&common.ConnectResult{
				Identifier:    ids,
				Status:        common.SUCCESS,
				Transmissions: []string{`{"type":"welcome","restored":false}`},
			}, nil)

		new_session := NewMockSessionWithEnv("s42", node, fmt.Sprintf("ws://test.anycable.io/cable?sid=%s", sid), nil)

		time.Sleep(4 * time.Second)

		_, err = node.Authenticate(new_session)
		require.NoError(t, err)

		requireReceive(
			t,
			new_session,
			`{"type":"welcome","restored":false}`,
		)
	})
}

// A test to verify the history flow.
//
// SETUP:
// - A session is created (authenticated).
// - A few broadcasts are made to ensure that the history is not empty.
//
// TEST 1 — subscribe with history:
// - A subscribe command with history request is made (with Since option).
// - The session MUST receive the confirmation and the backlog messages.
//
// TEST 2 — subscribe and history with offsets:
// - A subscribe request is made.
// - A few broadcasts are made.
// - The session MUST receive the messages.
// - The session unsubscribes.
// - More broadcasts are made.
// - The session subscribes again.
// - A history request is made with stream offsets.
// - The session MUST receive the messages broadcasted during the unsubsciprtion period.
func TestIntegrationHistory_Memory(t *testing.T) {
	node, controller := setupIntegrationNode()
	go node.Start()                           // nolint:errcheck
	defer node.Shutdown(context.Background()) // nolint:errcheck

	sharedIntegrationHistory(t, node, controller)
}

func sharedIntegrationHistory(t *testing.T, node *Node, controller *mocks.Controller) {
	node.HandleBroadcast([]byte(`{"stream": "messages_1","data":"Lorenzo: Ciao"}`))

	// Use sleep to make sure Since option works (and we don't want
	// to hack broker internals to update stream messages timestamps)
	time.Sleep(2 * time.Second)
	ts := time.Now().Unix()

	node.HandleBroadcast([]byte(`{"stream": "messages_1","data":"Flavia: buona sera"}`))
	node.HandleBroadcast([]byte(`{"stream": "messages_1","data":"Mario: ta-dam!"}`))

	node.HandleBroadcast([]byte(`{"stream": "presence_1","data":"1 new notification"}`))
	node.HandleBroadcast([]byte(`{"stream": "presence_1","data":"2 new notifications"}`))
	node.HandleBroadcast([]byte(`{"stream": "presence_1","data":"3 new notifications"}`))
	node.HandleBroadcast([]byte(`{"stream": "presence_1","data":"4 new notifications"}`))
	node.HandleBroadcast([]byte(`{"stream": "presence_1","data":"100+ new notifications"}`))

	t.Run("Subscribe with history", func(t *testing.T) {
		session := requireAuthenticatedSession(t, node, "alice")

		controller.
			On("Subscribe", "alice", mock.Anything, "alice", "chat_1").
			Return(&common.CommandResult{
				Status:        common.SUCCESS,
				Streams:       []string{"messages_1"},
				Transmissions: []string{`{"type":"confirm","identifier":"chat_1"}`},
			}, nil)

		_, err := node.Subscribe(
			session,
			&common.Message{
				Identifier: "chat_1",
				Command:    "subscribe",
				History: common.HistoryRequest{
					Since: ts,
				},
			})

		require.NoError(t, err)

		assertReceive(t, session, `{"type":"confirm","identifier":"chat_1"}`)
		assertReceive(t, session, `{"identifier":"chat_1","message":"Flavia: buona sera","stream_id":"messages_1","epoch":"2022","offset":2}`)
		assertReceive(t, session, `{"identifier":"chat_1","message":"Mario: ta-dam!","stream_id":"messages_1","epoch":"2022","offset":3}`)
		assertReceive(t, session, `{"type":"confirm_history","identifier":"chat_1"}`)
	})

	t.Run("Subscribe + History", func(t *testing.T) {
		session := requireAuthenticatedSession(t, node, "bob")

		controller.
			On("Subscribe", "bob", mock.Anything, "bob", "chat_1").
			Return(&common.CommandResult{
				Status:        common.SUCCESS,
				Streams:       []string{"messages_1", "presence_1"},
				Transmissions: []string{`{"type":"confirm","identifier":"chat_1"}`},
			}, nil)

		_, err := node.Subscribe(
			session,
			&common.Message{
				Identifier: "chat_1",
				Command:    "subscribe",
			})

		require.NoError(t, err)

		requireReceive(t, session, `{"type":"confirm","identifier":"chat_1"}`)

		err = node.History(
			session,
			&common.Message{
				Identifier: "chat_1",
				Command:    "history",
				History: common.HistoryRequest{
					Streams: map[string]common.HistoryPosition{
						"presence_1": {Epoch: "2022", Offset: 2},
					},
				},
			},
		)

		require.NoError(t, err)

		assertReceive(t, session, `{"identifier":"chat_1","message":"3 new notifications","stream_id":"presence_1","epoch":"2022","offset":3}`)
		assertReceive(t, session, `{"identifier":"chat_1","message":"4 new notifications","stream_id":"presence_1","epoch":"2022","offset":4}`)
		assertReceive(t, session, `{"identifier":"chat_1","message":"100+ new notifications","stream_id":"presence_1","epoch":"2022","offset":5}`)
		assertReceive(t, session, `{"type":"confirm_history","identifier":"chat_1"}`)
	})
}

func setupIntegrationNode() (*Node, *mocks.Controller) {
	config := NewConfig()
	config.HubGopoolSize = 2
	config.DisconnectMode = DISCONNECT_MODE_NEVER

	controller := &mocks.Controller{}
	controller.On("Shutdown").Return(nil)

	node := NewNode(controller, metrics.NewMetrics(nil, 10), &config)
	node.SetDisconnector(NewNoopDisconnector())

	bconf := broker.NewConfig()
	bconf.SessionsTTL = 2

	subscriber := pubsub.NewLegacySubscriber(node)

	br := broker.NewMemoryBroker(subscriber, &bconf)
	br.SetEpoch("2022")
	node.SetBroker(br)

	return node, controller
}

func requireReceive(t *testing.T, s *Session, expected string) {
	msg, err := s.conn.Read()
	require.NoError(t, err)

	require.Equal(
		t,
		expected,
		string(msg),
	)
}

func assertReceive(t *testing.T, s *Session, expected string) {
	msg, err := s.conn.Read()
	require.NoError(t, err)

	assert.Equal(
		t,
		expected,
		string(msg),
	)
}

func requireAuthenticatedSession(t *testing.T, node *Node, sid string) *Session {
	session := NewMockSessionWithEnv(sid, node, "ws://test.anycable.io/cable", nil)

	controller := node.controller.(*mocks.Controller)

	controller.
		On("Authenticate", sid, session.env).
		Return(&common.ConnectResult{
			Identifier:    sid,
			Status:        common.SUCCESS,
			Transmissions: []string{`{"type":"welcome"}`},
		}, nil)

	_, err := node.Authenticate(session)
	require.NoError(t, err)

	requireReceive(
		t,
		session,
		`{"type":"welcome"}`,
	)

	return session
}
