package node

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/mocks"
	"github.com/joomcode/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAuthenticate(t *testing.T) {
	node := NewMockNode()
	go node.hub.Run()
	defer node.hub.Shutdown()

	t.Run("Successful authentication", func(t *testing.T) {
		session := NewMockSessionWithEnv("1", node, "/cable", &map[string]string{"id": "test_id"})
		_, err := node.Authenticate(session)
		defer node.hub.RemoveSession(session)

		assert.Nil(t, err, "Error must be nil")
		assert.Equal(t, true, session.Connected, "Session must be marked as connected")
		assert.Equalf(t, "test_id", session.GetIdentifiers(), "Identifiers must be equal to %s", "test_id")

		msg, err := session.conn.Read()
		assert.Nil(t, err)

		assert.Equalf(t, []byte("welcome"), msg, "Sent message is invalid: %s", msg)

		assert.Equal(t, 1, node.hub.Size())
	})

	t.Run("Failed authentication", func(t *testing.T) {
		session := NewMockSessionWithEnv("1", node, "/failure", &map[string]string{"id": "test_id"})

		_, err := node.Authenticate(session)

		assert.Nil(t, err, "Error must be nil")

		msg, err := session.conn.Read()
		assert.Nil(t, err)

		assert.Equalf(t, []byte("unauthorized"), msg, "Sent message is invalid: %s", msg)
		assert.Equal(t, 0, node.hub.Size())
	})

	t.Run("Error during authentication", func(t *testing.T) {
		session := NewMockSessionWithEnv("1", node, "/error", &map[string]string{"id": "test_id"})

		_, err := node.Authenticate(session)

		assert.NotNil(t, err, "Error must not be nil")
		assert.Equal(t, 0, node.hub.Size())
	})

	t.Run("With connection state", func(t *testing.T) {
		session := NewMockSessionWithEnv("1", node, "/cable", &map[string]string{"x-session-test": "my_session", "id": "session_id"})
		defer node.hub.RemoveSession(session)

		_, err := node.Authenticate(session)

		assert.Nil(t, err, "Error must be nil")
		assert.Equal(t, true, session.Connected, "Session must be marked as connected")

		assert.Len(t, *session.env.ConnectionState, 1)
		assert.Equal(t, "my_session", (*session.env.ConnectionState)["_s_"])

		assert.Equal(t, 1, node.hub.Size())
	})
}

func TestSubscribe(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14", node)

	node.hub.AddSession(session)
	defer node.hub.RemoveSession(session)

	go node.hub.Run()
	defer node.hub.Shutdown()

	t.Run("Successful subscription", func(t *testing.T) {
		_, err := node.Subscribe(session, &common.Message{Identifier: "test_channel"})
		assert.Nil(t, err, "Error must be nil")

		// Adds subscription to session
		assert.Truef(t, session.subscriptions.HasChannel("test_channel"), "Session subscription must be set")

		msg, err := session.conn.Read()
		assert.Nil(t, err)

		assert.Equalf(t, []byte("14"), msg, "Sent message is invalid: %s", msg)
	})

	t.Run("Subscription with a stream", func(t *testing.T) {
		_, err := node.Subscribe(session, &common.Message{Identifier: "with_stream"})
		assert.Nil(t, err, "Error must be nil")

		// Adds subscription and stream to session
		assert.Truef(t, session.subscriptions.HasChannel("with_stream"), "Session subsription must be set")
		assert.Equal(t, []string{"stream"}, session.subscriptions.StreamsFor("with_stream"))

		msg, err := session.conn.Read()
		assert.Nil(t, err)

		assert.Equalf(t, "14", string(msg), "Sent message is invalid: %s", msg)

		// Make sure session is subscribed
		node.hub.BroadcastMessage(&common.StreamMessage{Stream: "stream", Data: "41"})

		msg, err = session.conn.Read()
		assert.Nil(t, err)

		assert.Equalf(t, "{\"identifier\":\"with_stream\",\"message\":41}", string(msg), "Broadcasted message is invalid: %s", msg)
	})

	t.Run("Error during subscription", func(t *testing.T) {
		_, err := node.Subscribe(session, &common.Message{Identifier: "error"})
		assert.Error(t, err)
	})

	t.Run("Rejected subscription", func(t *testing.T) {
		session := NewMockSession("15", node)

		node.hub.AddSession(session)
		defer node.hub.RemoveSession(session)

		res, err := node.Subscribe(session, &common.Message{Identifier: "failure"})

		assert.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, 0, len(session.subscriptions.Channels()))
		assert.NoError(t, err)
	})
}

func TestUnsubscribe(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14", node)

	node.hub.AddSession(session)
	defer node.hub.RemoveSession(session)

	go node.hub.Run()
	defer node.hub.Shutdown()

	t.Run("Successful unsubscribe", func(t *testing.T) {
		session.subscriptions.AddChannel("test_channel")
		node.hub.SubscribeSession(session, "streamo", "test_channel")

		node.hub.Broadcast("streamo", `"before"`)
		msg, err := session.conn.Read()
		require.NoError(t, err)

		assert.Equalf(t, `{"identifier":"test_channel","message":"before"}`, string(msg), "Broadcasted message is invalid: %s", msg)

		_, err = node.Unsubscribe(session, &common.Message{Identifier: "test_channel"})
		assert.Nil(t, err, "Error must be nil")

		// Removes subscription from session
		assert.Falsef(t, session.subscriptions.HasChannel("test_channel"), "Shouldn't contain test_channel")

		node.hub.BroadcastMessage(&common.StreamMessage{Stream: "streamo", Data: "41"})

		msg, err = session.conn.Read()
		assert.Nil(t, msg)
		assert.Error(t, err, "Session hasn't received any messages")
	})

	t.Run("Error during unsubscription", func(t *testing.T) {
		session.subscriptions.AddChannel("failure")

		_, err := node.Unsubscribe(session, &common.Message{Identifier: "failure"})
		assert.Error(t, err)
	})
}

func TestPerform(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14", node)

	node.hub.AddSession(session)
	defer node.hub.RemoveSession(session)

	session.subscriptions.AddChannel("test_channel")

	go node.hub.Run()
	defer node.hub.Shutdown()

	t.Run("Successful perform", func(t *testing.T) {
		_, err := node.Perform(session, &common.Message{Identifier: "test_channel", Data: "action"})
		assert.Nil(t, err)

		msg, err := session.conn.Read()
		assert.Nil(t, err)

		assert.Equalf(t, []byte("action"), msg, "Sent message is invalid: %s", msg)
	})

	t.Run("With connection state", func(t *testing.T) {
		_, err := node.Perform(session, &common.Message{Identifier: "test_channel", Data: "session"})
		assert.Nil(t, err)

		_, err = session.conn.Read()
		assert.Nil(t, err)

		assert.Len(t, *session.env.ConnectionState, 1)
		assert.Equal(t, "performed", (*session.env.ConnectionState)["_s_"])
	})

	t.Run("Error during perform", func(t *testing.T) {
		session.subscriptions.AddChannel("failure")

		_, err := node.Perform(session, &common.Message{Identifier: "failure", Data: "test"})
		assert.NotNil(t, err, "Error must not be nil")
	})

	t.Run("With stopped streams", func(t *testing.T) {
		session.subscriptions.AddChannelStream("test_channel", "stop_stream")
		node.hub.SubscribeSession(session, "stop_stream", "test_channel")

		node.hub.BroadcastMessage(&common.StreamMessage{Stream: "stop_stream", Data: "40"})

		msg, _ := session.conn.Read()
		assert.NotNil(t, msg)

		_, err := node.Perform(session, &common.Message{Identifier: "test_channel", Data: "stop_stream"})
		assert.Nil(t, err)

		assert.Empty(t, session.subscriptions.StreamsFor("test_channel"))

		_, err = node.Perform(session, &common.Message{Identifier: "test_channel", Data: "stop_stream"})
		assert.Nil(t, err)

		node.hub.BroadcastMessage(&common.StreamMessage{Stream: "stop_stream", Data: "41"})

		msg, err = session.conn.Read()
		assert.Nil(t, msg)
		assert.Error(t, err, "Session hasn't received any messages")
	})

	t.Run("With channel state", func(t *testing.T) {
		assert.Len(t, *session.env.ChannelStates, 0)

		_, err := node.Perform(session, &common.Message{Identifier: "test_channel", Data: "channel_state"})
		assert.Nil(t, err)

		_, err = session.conn.Read()
		assert.Nil(t, err)

		assert.Len(t, *session.env.ChannelStates, 1)
		assert.Len(t, (*session.env.ChannelStates)["test_channel"], 1)
		assert.Equal(t, "performed", (*session.env.ChannelStates)["test_channel"]["_c_"])
	})
}

func TestWhisper(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14", node)
	session2 := NewMockSession("15", node)

	// Subscribe using different identifiers to make sure whisper is working
	// per stream name, not per identifier
	defer subscribeSessionToStream(session, node, "test_channel", "test_whisper")()
	defer subscribeSessionToStream(session2, node, "test_channel_2", "test_whisper")()

	go node.hub.Run()
	defer node.hub.Shutdown()

	t.Run("When whispering stream is configured for sending subscription", func(t *testing.T) {
		session.env.MergeChannelState("test_channel", &map[string]string{common.WHISPER_STREAM_STATE: "test_whisper"})

		err := node.Whisper(session, &common.Message{Identifier: "test_channel", Data: "tshh... it's a secret"})
		assert.Nil(t, err)

		expected := `{"identifier":"test_channel_2","message":"tshh... it's a secret"}`

		msg, err := session2.conn.Read()
		assert.NoError(t, err)
		assert.Equal(t, expected, string(msg))

		// Sender do not receive the message
		msg, err = session.conn.Read()
		assert.Nil(t, msg)
		assert.Error(t, err, "Session hasn't received any messages")
	})

	t.Run("When whispering stream is not configured", func(t *testing.T) {
		session.env.RemoveChannelState("test_channel")

		err := node.Whisper(session, &common.Message{Identifier: "test_channel", Data: "tshh... it's a secret"})
		assert.Nil(t, err)

		msg, err := session2.conn.Read()
		assert.Error(t, err)
		assert.Nil(t, msg)

		// Sender do not receive the message
		msg, err = session.conn.Read()
		assert.Nil(t, msg)
		assert.Error(t, err, "Session hasn't received any messages")
	})
}

func TestStreamSubscriptionRaceConditions(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14", node)

	node.hub.AddSession(session)
	session.subscriptions.AddChannel("test_channel")

	// We need a real hub here to catch a race condition
	go node.hub.Run()
	defer node.hub.Shutdown()

	t.Run("stop and start streams race conditions", func(t *testing.T) {
		_, err := node.Perform(session, &common.Message{Identifier: "test_channel", Data: "stop_and_start_streams"})
		assert.Nil(t, err)

		// Make sure session is subscribed to the stream
		node.hub.Broadcast("all", "2022")

		msg, err := session.conn.Read()
		require.NoError(t, err)

		assert.Equalf(t, "{\"identifier\":\"test_channel\",\"message\":2022}", string(msg), "Broadcasted message is invalid: %s", msg)
	})
}

func TestDisconnect(t *testing.T) {
	node := NewMockNode()
	go node.hub.Run()
	defer node.hub.Shutdown()

	t.Run("Disconnectable session", func(t *testing.T) {
		session := NewMockSessionWithEnv("1", node, "/cable", &map[string]string{"id": "test_id"})
		// Authenticate via controller marks session as disconnectable automatically
		_, err := node.Authenticate(session)
		require.NoError(t, err)

		assert.True(t, session.IsDisconnectable())

		assert.Nil(t, node.Disconnect(session))

		assert.Equal(t, 1, node.disconnector.Size(), "Expected disconnect to have 1 task in a queue")

		task := <-node.disconnector.(*DisconnectQueue).disconnect
		assert.Equal(t, session, task, "Expected to disconnect session")
	})

	t.Run("Non-disconnectable session", func(t *testing.T) {
		session := NewMockSessionWithEnv("1", node, "/cable", &map[string]string{"id": "test_id"})
		// Authenticate via controller marks session as disconnectable automatically
		_, err := node.Authenticate(session)
		require.NoError(t, err)

		session.disconnectInterest = false

		assert.Nil(t, node.Disconnect(session))

		assert.Equal(t, 0, node.disconnector.Size(), "Expected disconnect to have 0 tasks in a queue")
	})
}

func TestHistory(t *testing.T) {
	node := NewMockNode()

	broker := &mocks.Broker{}
	node.SetBroker(broker)

	broker.
		On("CommitSession", mock.Anything, mock.Anything).
		Return(nil)

	go node.hub.Run()
	defer node.hub.Shutdown()

	session := NewMockSession("14", node)

	session.subscriptions.AddChannel("test_channel")
	session.subscriptions.AddChannelStream("test_channel", "streamo")
	session.subscriptions.AddChannelStream("test_channel", "emptissimo")

	stream := []common.StreamMessage{
		{
			Stream: "streamo",
			Data:   "ciao",
			Offset: 22,
			Epoch:  "test",
		},
		{
			Stream: "streamo",
			Data:   "buona sera",
			Offset: 23,
			Epoch:  "test",
		},
	}

	var ts int64

	t.Run("Successful history with only Since", func(t *testing.T) {
		ts = 100200

		broker.
			On("HistorySince", "streamo", ts).
			Return(stream, nil)
		broker.
			On("HistorySince", "emptissimo", ts).
			Return(nil, nil)

		err := node.History(
			session,
			&common.Message{
				Identifier: "test_channel",
				History: common.HistoryRequest{
					Since: ts,
				},
			},
		)
		require.NoError(t, err)

		history := []string{
			"{\"identifier\":\"test_channel\",\"message\":\"ciao\",\"stream_id\":\"streamo\",\"epoch\":\"test\",\"offset\":22}",
			"{\"identifier\":\"test_channel\",\"message\":\"buona sera\",\"stream_id\":\"streamo\",\"epoch\":\"test\",\"offset\":23}",
		}

		for _, msg := range history {
			received, herr := session.conn.Read()
			require.NoError(t, herr)

			require.Equalf(
				t,
				msg,
				string(received),
				"Sent message is invalid: %s", received,
			)
		}

		ack, err := session.conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `{"type":"confirm_history","identifier":"test_channel"}`, string(ack))

		_, err = session.conn.Read()
		require.Error(t, err)
	})

	t.Run("Successful history with Since and Offset", func(t *testing.T) {
		ts = 100300

		broker.
			On("HistoryFrom", "streamo", "test", uint64(20)).
			Return(stream, nil)
		broker.
			On("HistorySince", "emptissimo", ts).
			Return([]common.StreamMessage{{
				Stream: "emptissimo",
				Data:   "zer0",
				Offset: 2,
				Epoch:  "test_zero",
			}}, nil)

		err := node.History(
			session,
			&common.Message{
				Identifier: "test_channel",
				History: common.HistoryRequest{
					Since: ts,
					Streams: map[string]common.HistoryPosition{
						"streamo": {Epoch: "test", Offset: 20},
					},
				},
			},
		)
		require.NoError(t, err)

		history := []string{
			"{\"identifier\":\"test_channel\",\"message\":\"ciao\",\"stream_id\":\"streamo\",\"epoch\":\"test\",\"offset\":22}",
			"{\"identifier\":\"test_channel\",\"message\":\"buona sera\",\"stream_id\":\"streamo\",\"epoch\":\"test\",\"offset\":23}",
			"{\"identifier\":\"test_channel\",\"message\":\"zer0\",\"stream_id\":\"emptissimo\",\"epoch\":\"test_zero\",\"offset\":2}",
		}

		// The order of streams is non-deterministic, so
		// we're collecting messages first and checking for inclusion later
		received := []string{}

		for range history {
			data, herr := session.conn.Read()
			require.NoError(t, herr)

			received = append(received, string(data))
		}

		for _, msg := range history {
			require.Contains(
				t,
				received,
				msg,
			)
		}

		ack, err := session.conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `{"type":"confirm_history","identifier":"test_channel"}`, string(ack))

		_, err = session.conn.Read()
		require.Error(t, err)
	})

	t.Run("Fetching history with Subscribe", func(t *testing.T) {
		ts = 100400

		broker.
			On("HistoryFrom", "streamo", "test", uint64(21)).
			Return(stream, nil)
		broker.
			On("HistorySince", "s1", ts).
			Return([]common.StreamMessage{{
				Stream: "s1",
				Data:   "{\"foo\":\"bar\"}",
				Offset: 10,
				Epoch:  "test",
			}}, nil)
		broker.
			On("Subscribe", "stream").
			Return("s1")

		_, err := node.Subscribe(
			session,
			&common.Message{
				Identifier: "with_stream",
				History: common.HistoryRequest{
					Since: ts,
					Streams: map[string]common.HistoryPosition{
						"streamo": {Epoch: "test", Offset: 21},
					},
				},
			},
		)
		require.NoError(t, err)

		msg, err := session.conn.Read()
		require.NoError(t, err)

		require.Equalf(t, "14", string(msg), "Sent message is invalid: %s", msg)

		history := []string{
			"{\"identifier\":\"with_stream\",\"message\":{\"foo\":\"bar\"},\"stream_id\":\"s1\",\"epoch\":\"test\",\"offset\":10}",
		}

		for _, msg := range history {
			received, herr := session.conn.Read()
			require.NoError(t, herr)

			require.Equalf(
				t,
				msg,
				string(received),
				"Sent message is invalid: %s", received,
			)
		}

		ack, err := session.conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `{"type":"confirm_history","identifier":"with_stream"}`, string(ack))

		_, err = session.conn.Read()
		require.Error(t, err)
	})

	t.Run("Error retrieving history", func(t *testing.T) {
		ts = 200100

		broker.
			On("HistorySince", "streamo", ts).
			Return(nil, errors.New("Couldn't restore history"))
		broker.
			On("HistorySince", "emptissimo", ts).
			Return(stream, nil)

		err := node.History(
			session,
			&common.Message{
				Identifier: "test_channel",
				History: common.HistoryRequest{
					Since: ts,
				},
			},
		)

		assert.Error(t, err, "Couldn't restore history")

		ack, err := session.conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `{"type":"reject_history","identifier":"test_channel"}`, string(ack))
	})
}

func TestPresence(t *testing.T) {
	node := NewMockNode()

	controller := &mocks.Controller{}
	node.controller = controller

	broker := &mocks.Broker{}
	node.SetBroker(broker)

	broker.
		On("CommitSession", mock.Anything, mock.Anything).
		Return(nil)

	broker.
		On("Subscribe", mock.Anything).
		Return(func(name string) string { return name })

	go node.hub.Run()
	defer node.hub.Shutdown()

	t.Run("join on subscribe", func(t *testing.T) {
		session := NewMockSession("24", node)

		node.hub.AddSession(session)
		defer node.hub.RemoveSession(session)

		controller.
			On("Subscribe", "24", mock.Anything, "24", "test_channel").
			Return(&common.CommandResult{
				Status:        common.SUCCESS,
				Transmissions: []string{`{"type":"confirm","identifier":"test_channel"}`},
				Streams:       []string{"test_presence"},
				IState:        map[string]string{common.PRESENCE_STREAM_STATE: "test_presence"},
				Presence: &common.PresenceEvent{
					Type: common.PresenceJoinType,
					ID:   "user_24",
					Info: "J. Bond",
				},
			}, nil)

		broker.On(
			"PresenceAdd", "test_presence", "24", "user_24", "J. Bond",
		).Return(&common.PresenceEvent{
			Type: "join",
			ID:   "user_24",
			Info: "J. Bond",
		}, nil)

		_, err := node.Subscribe(session, &common.Message{Identifier: "test_channel"})
		require.NoError(t, err)

		assert.Equal(t, "test_presence", session.GetEnv().GetChannelStateField("test_channel", common.PRESENCE_STREAM_STATE))

		assertReceive(t, session, `{"type":"confirm","identifier":"test_channel"}`)

		assertReceive(t, session, `{"type":"presence","identifier":"test_channel","message":{"type":"join","id":"user_24","info":"J. Bond"}}`)
	})

	t.Run("join on perform", func(t *testing.T) {
		session := NewMockSession("24", node)

		node.hub.AddSession(session)
		defer node.hub.RemoveSession(session)

		session.subscriptions.AddChannel("subscribed_channel")

		controller.
			On("Perform", "24", mock.Anything, "24", "subscribed_channel", `{"action":"follow"}`).
			Return(&common.CommandResult{
				Status:  common.SUCCESS,
				Streams: []string{"test_presence"},
				IState:  map[string]string{common.PRESENCE_STREAM_STATE: "test_presence"},
				Presence: &common.PresenceEvent{
					Type: common.PresenceJoinType,
					ID:   "user_24",
					Info: "J. Bond",
				},
			}, nil)

		broker.On(
			"PresenceAdd", "test_presence", "24", "user_24", "J. Bond",
		).Return(&common.PresenceEvent{
			Type: "join",
			ID:   "user_24",
			Info: "J. Bond",
		}, nil)

		_, err := node.Perform(session, &common.Message{Identifier: "subscribed_channel", Data: `{"action":"follow"}`})
		require.NoError(t, err)

		assert.Equal(t, "test_presence", session.GetEnv().GetChannelStateField("subscribed_channel", common.PRESENCE_STREAM_STATE))

		assertReceive(t, session, `{"type":"presence","identifier":"subscribed_channel","message":{"type":"join","id":"user_24","info":"J. Bond"}}`)
	})

	t.Run("no presence stream configured", func(t *testing.T) {
		session := NewMockSession("25", node)

		node.hub.AddSession(session)
		defer node.hub.RemoveSession(session)

		session.subscriptions.AddChannel("subscribed_channel")

		controller.
			On("Perform", "25", mock.Anything, "25", "subscribed_channel", `{"action":"follow"}`).
			Return(&common.CommandResult{
				Status:  common.SUCCESS,
				Streams: []string{"test_presence"},
				Presence: &common.PresenceEvent{
					Type: common.PresenceJoinType,
					ID:   "user_25",
					Info: "Fantomas",
				},
			}, nil)

		broker.On(
			"PresenceAdd", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Run(func(args mock.Arguments) {
			t.Error("PresenceAdd should not be called")
		})

		_, err := node.Perform(session, &common.Message{Identifier: "subscribed_channel", Data: `{"action":"follow"}`})
		require.NoError(t, err)

		assert.Equal(t, "", session.GetEnv().GetChannelStateField("subscribed_channel", common.PRESENCE_STREAM_STATE))

		_, err = session.conn.Read()
		assert.Error(t, err)
	})
}

func TestRestoreSession(t *testing.T) {
	node := NewMockNode()

	broker := &mocks.Broker{}
	node.SetBroker(broker)

	go node.hub.Run()
	defer node.hub.Shutdown()

	prev_session := NewMockSession("114", node, WithKeepaliveIntervals(500*time.Millisecond, 500*time.Millisecond))
	prev_session.subscriptions.AddChannel("fruits_channel")
	prev_session.subscriptions.AddChannelStream("fruits_channel", "arancia")
	prev_session.subscriptions.AddChannelStream("fruits_channel", "limoni")
	prev_session.env.MergeConnectionState(&(map[string]string{"tenant_id": "71"}))
	prev_session.env.MergeChannelState("fruits_channel", &(map[string]string{"gardini": "ischia"}))

	cached, err := prev_session.ToCacheEntry()
	require.NoError(t, err)

	broker.
		On("RestoreSession", "114").
		Return(cached, nil)
	broker.
		On("CommitSession", mock.Anything, mock.Anything).
		Return(nil)
	broker.
		On("Subscribe", mock.Anything).
		Return(func(name string) string { return name })

	session := NewMockSession("214", node, WithKeepaliveIntervals(500*time.Millisecond, 500*time.Millisecond), WithPrevSID("114"))

	t.Run("Successful restore via header", func(t *testing.T) {
		res, err := node.Authenticate(session)
		require.NoError(t, err)
		assert.Equal(t, common.SUCCESS, res.Status)

		assert.Contains(t, session.subscriptions.StreamsFor("fruits_channel"), "arancia")
		assert.Contains(t, session.subscriptions.StreamsFor("fruits_channel"), "limoni")

		assert.Equal(t, "71", session.env.GetConnectionStateField("tenant_id"))
		assert.Equal(t, "ischia", session.env.GetChannelStateField("fruits_channel", "gardini"))

		welcome, err := session.conn.Read()
		require.NoError(t, err)

		require.Equalf(
			t,
			`{"type":"welcome","sid":"214","restored":true,"restored_ids":["fruits_channel"]}`,
			string(welcome),
			"Sent message is invalid: %s", welcome,
		)

		node.hub.Broadcast("arancia", "delissimo")

		msg, err := session.conn.Read()
		require.NoError(t, err)

		require.Equalf(
			t,
			`{"identifier":"fruits_channel","message":"delissimo"}`,
			string(msg),
			"Sent message is invalid: %s", msg,
		)

		node.hub.Broadcast("limoni", "acido")

		msg, err = session.conn.Read()
		require.NoError(t, err)

		require.Equalf(
			t,
			`{"identifier":"fruits_channel","message":"acido"}`,
			string(msg),
			"Sent message is invalid: %s", msg,
		)
	})

	t.Run("Failed to restore", func(t *testing.T) {
		broker.
			On("RestoreSession", "114").
			Return(nil, nil)

		session = NewMockSession("154", node)

		res, err := node.Authenticate(session)
		require.NoError(t, err)
		assert.Equal(t, common.SUCCESS, res.Status)

		welcome, err := session.conn.Read()
		require.NoError(t, err)

		require.Equalf(
			t,
			"welcome",
			string(welcome),
			"Sent message is invalid: %s", welcome,
		)
	})
}

func TestBroadcasting(t *testing.T) {
	node := NewMockNode()

	go node.hub.Run()
	defer node.hub.Shutdown()

	session := NewMockSession("14", node)
	session2 := NewMockSession("15", node)

	node.hub.AddSession(session)
	node.hub.SubscribeSession(session, "test", "test_channel")
	node.hub.SubscribeSession(session, "staind_2023", "music_channel")

	node.hub.AddSession(session2)
	node.hub.SubscribeSession(session2, "test", "test_channel")
	node.hub.SubscribeSession(session2, "staind_2023", "music_channel")

	node.broker.Subscribe("test")
	node.broker.Subscribe("staind_2023")

	t.Run("HandlePubSub", func(t *testing.T) {
		node.HandlePubSub([]byte(`{"stream":"test","data":"\"abc123\""}`))

		expected := `{"identifier":"test_channel","message":"abc123"}`

		msg, err := session.conn.Read()
		assert.Nil(t, err)
		assert.Equalf(t, expected, string(msg), "Expected to receive %s but got %s", expected, string(msg))

		msg2, err := session2.conn.Read()
		assert.Nil(t, err)
		assert.Equalf(t, expected, string(msg2), "Expected to receive %s but got %s", expected, string(msg2))
	})

	t.Run("HandlePubSub_Batch", func(t *testing.T) {
		node.HandlePubSub([]byte(`[{"stream":"test","data":"\"follow me\""},{"stream":"untest","data":"\"missing\""},{"stream":"staind_2023","data":"{\"num\":7,\"title\":\"The Fray\"}"}]`))

		first := `{"identifier":"test_channel","message":"follow me"}`
		second := `{"identifier":"music_channel","message":{"num":7,"title":"The Fray"}}`

		msgs, err := readMessages(session.conn, 2)
		assert.Nil(t, err)
		assert.Contains(t, msgs, first)
		assert.Contains(t, msgs, second)

		msgs2, err := readMessages(session2.conn, 2)
		assert.Nil(t, err)
		assert.Contains(t, msgs2, first)
		assert.Contains(t, msgs2, second)
	})

	t.Run("HandleBroadcast", func(t *testing.T) {
		node.HandleBroadcast([]byte(`{"stream":"staind_2023","data":"{\"num\":5,\"title\":\"Out of time\"}"}`))

		expected := `{"identifier":"music_channel","message":{"num":5,"title":"Out of time"}}`

		msg, err := session.conn.Read()
		assert.Nil(t, err)
		assert.Equalf(t, expected, string(msg), "Expected to receive %s but got %s", expected, string(msg))

		msg2, err := session2.conn.Read()
		assert.Nil(t, err)
		assert.Equalf(t, expected, string(msg2), "Expected to receive %s but got %s", expected, string(msg2))
	})

	t.Run("HandleBroadcast_Batch", func(t *testing.T) {
		node.HandleBroadcast([]byte(`[{"stream":"staind_2023","data":"{\"num\":9,\"title\":\"Hate me too\"}"},{"stream":"untest","data":"\"missing\""},{"stream":"staind_2023","data":"{\"num\":10,\"title\":\"Confessions of Fallen\"}"}]`))

		first := `{"identifier":"music_channel","message":{"num":9,"title":"Hate me too"}}`
		second := `{"identifier":"music_channel","message":{"num":10,"title":"Confessions of Fallen"}}`

		msgs, err := readMessages(session.conn, 2)
		assert.Nil(t, err)
		assert.Contains(t, msgs, first)
		assert.Contains(t, msgs, second)

		msgs2, err := readMessages(session2.conn, 2)
		assert.Nil(t, err)
		assert.Contains(t, msgs2, first)
		assert.Contains(t, msgs2, second)
	})
}

func TestHandlePubSubWithCommand(t *testing.T) {
	node := NewMockNode()

	go node.hub.Run()
	defer node.hub.Shutdown()

	session := NewMockSession("14", node)
	node.hub.AddSession(session)

	node.HandlePubSub([]byte("{\"command\":\"disconnect\",\"payload\":{\"identifier\":\"14\",\"reconnect\":false}}"))

	expected := string(toJSON(common.NewDisconnectMessage("remote", false)))

	msg, err := session.conn.Read()
	assert.Nil(t, err)
	assert.Equalf(t, expected, string(msg), "Expected to receive %s but got %s", expected, string(msg))
	assert.True(t, session.closed)
}

func TestLookupSession(t *testing.T) {
	node := NewMockNode()

	go node.hub.Run()
	defer node.hub.Shutdown()

	assert.Nil(t, node.LookupSession("{\"foo\":\"bar\"}"))

	session := NewMockSession("14", node)
	session.SetIdentifiers("{\"foo\":\"bar\"}")
	node.hub.AddSession(session)

	assert.Equal(t, session, node.LookupSession("{\"foo\":\"bar\"}"))
}

func toJSON(msg encoders.EncodedMessage) []byte {
	b, err := json.Marshal(&msg)
	if err != nil {
		panic("Failed to build JSON 😲")
	}

	return b
}

func subscribeSessionToStream(s *Session, n *Node, identifier string, stream string) func() {
	n.hub.AddSession(s)

	s.subscriptions.AddChannel(identifier)
	s.subscriptions.AddChannelStream(identifier, stream)
	n.hub.SubscribeSession(s, stream, identifier)
	n.broker.Subscribe(stream)

	return func() {
		n.hub.RemoveSession(s)
	}
}

func readMessages(conn Connection, count int) ([]string, error) {
	var messages []string

	for i := 0; i < count; i++ {
		msg, err := conn.Read()
		if err != nil {
			if len(messages) > 0 {
				return messages, errorx.Decorate(err, "received only %d messsages", len(messages))
			}
			return nil, err
		}

		messages = append(messages, string(msg))
	}

	return messages, nil
}
