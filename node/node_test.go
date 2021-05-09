package node

import (
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
)

func TestAuthenticate(t *testing.T) {
	node := NewMockNode()

	t.Run("Successful authentication", func(t *testing.T) {
		session := NewMockSessionWithEnv("1", &node, "/cable", &map[string]string{"id": "test_id"})
		_, err := node.Authenticate(session)

		assert.Nil(t, err, "Error must be nil")
		assert.Equal(t, true, session.connected, "Session must be marked as connected")
		assert.Equalf(t, "test_id", session.Identifiers, "Identifiers must be equal to %s", "test_id")

		msg, err := session.conn.Read()
		assert.Nil(t, err)

		assert.Equalf(t, []byte("welcome"), msg, "Sent message is invalid: %s", msg)
	})

	t.Run("Failed authentication", func(t *testing.T) {
		session := NewMockSessionWithEnv("1", &node, "/failure", &map[string]string{"id": "test_id"})

		_, err := node.Authenticate(session)

		assert.NotNil(t, err, "Error must not be nil")

		msg, err := session.conn.Read()
		assert.Nil(t, err)

		assert.Equalf(t, []byte("unauthorized"), msg, "Sent message is invalid: %s", msg)
	})

	t.Run("Error during authentication", func(t *testing.T) {
		session := NewMockSessionWithEnv("1", &node, "/error", &map[string]string{"id": "test_id"})

		_, err := node.Authenticate(session)

		assert.NotNil(t, err, "Error must not be nil")
	})

	t.Run("With connection state", func(t *testing.T) {
		session := NewMockSessionWithEnv("1", &node, "/cable", &map[string]string{"x-session-test": "my_session", "id": "session_id"})

		_, err := node.Authenticate(session)

		assert.Nil(t, err, "Error must be nil")
		assert.Equal(t, true, session.connected, "Session must be marked as connected")

		assert.Len(t, *session.env.ConnectionState, 1)
		assert.Equal(t, "my_session", (*session.env.ConnectionState)["_s_"])
	})
}

func TestSubscribe(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14", &node)

	t.Run("Successful subscription", func(t *testing.T) {
		_, err := node.Subscribe(session, &common.Message{Identifier: "test_channel"})
		assert.Nil(t, err, "Error must be nil")

		// Adds subscription to session
		assert.Contains(t, session.subscriptions, "test_channel", "Session subscription must be set")

		msg, err := session.conn.Read()
		assert.Nil(t, err)

		assert.Equalf(t, []byte("14"), msg, "Sent message is invalid: %s", msg)
	})

	t.Run("Subscription with a stream", func(t *testing.T) {
		_, err := node.Subscribe(session, &common.Message{Identifier: "stream"})
		assert.Nil(t, err, "Error must be nil")

		// Adds subscription to session
		assert.Contains(t, session.subscriptions, "stream", "Session subsription must be set")

		var subscription HubSubscription

		// Expected to subscribe session to hub
		select {
		case subscription = <-node.hub.subscribe:
		default:
			assert.Fail(t, "Expected hub to receive subscribe message but none was sent")
		}

		assert.Equalf(t, "14", subscription.session, "Session is invalid: %s", subscription.session)
		assert.Equalf(t, "stream", subscription.identifier, "Channel is invalid: %s", subscription.identifier)
		assert.Equalf(t, "stream", subscription.stream, "Stream is invalid: %s", subscription.stream)

		msg, err := session.conn.Read()
		assert.Nil(t, err)

		assert.Equalf(t, []byte("14"), msg, "Sent message is invalid: %s", msg)
	})

	t.Run("Error during subscription", func(t *testing.T) {
		_, err := node.Subscribe(session, &common.Message{Identifier: "failure"})
		assert.NotNil(t, err, "Error must not be nil")
	})
}

func TestUnsubscribe(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14", &node)

	t.Run("Sucessful unsubscribe", func(t *testing.T) {
		session.subscriptions["test_channel"] = true

		_, err := node.Unsubscribe(session, &common.Message{Identifier: "test_channel"})
		assert.Nil(t, err, "Error must be nil")

		// Removes subscription from session
		assert.NotContains(t, session.subscriptions, "test_channel", "Shouldn't contain test_channel")

		var subscription HubSubscription

		// Expected to subscribe session to hub
		select {
		case subscription = <-node.hub.subscribe:
		default:
			assert.Fail(t, "Expected hub to receive unsubscribe message but none was sent")
		}

		assert.Equalf(t, "14", subscription.session, "Session is invalid: %s", subscription.session)
		assert.Equalf(t, "test_channel", subscription.identifier, "Channel is invalid: %s", subscription.identifier)

		msg, err := session.conn.Read()
		assert.Nil(t, err)

		assert.Equalf(t, []byte("14"), msg, "Sent message is invalid: %s", msg)
	})

	t.Run("Error during unsubscription", func(t *testing.T) {
		session.subscriptions["failure"] = true

		_, err := node.Unsubscribe(session, &common.Message{Identifier: "failure"})
		assert.NotNil(t, err, "Error must not be nil")
	})
}

func TestPerform(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14", &node)

	session.subscriptions["test_channel"] = true

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
		session.subscriptions["failure"] = true

		_, err := node.Perform(session, &common.Message{Identifier: "failure", Data: "test"})
		assert.NotNil(t, err, "Error must not be nil")
	})

	t.Run("With stopped streams", func(t *testing.T) {
		_, err := node.Perform(session, &common.Message{Identifier: "test_channel", Data: "stop_stream"})
		assert.Nil(t, err)

		var subscription HubSubscription

		// Expected to subscribe session to hub
		select {
		case subscription = <-node.hub.subscribe:
		default:
			assert.Fail(t, "Expected hub to receive unsubscribe message but none was sent")
			return
		}

		_, err = session.conn.Read()
		assert.Nil(t, err)

		assert.Equalf(t, "14", subscription.session, "Session is invalid: %s", subscription.session)
		assert.Equalf(t, "test_channel", subscription.identifier, "Channel is invalid: %s", subscription.identifier)
		assert.Equalf(t, "stop_stream", subscription.stream, "Stream is invalid: %s", subscription.stream)
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

func TestDisconnect(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14", &node)

	assert.Nil(t, node.Disconnect(session))

	// Expected to unregister session
	select {
	case info := <-node.hub.register:
		assert.Equal(t, session, info.session, "Expected to disconnect session")
	default:
		assert.Fail(t, "Expected hub to receive unregister message but none was sent")
	}

	assert.Equal(t, node.disconnector.Size(), 1, "Expected disconnect to have 1 task in a queue")

	task := <-node.disconnector.(*DisconnectQueue).disconnect
	assert.Equal(t, session, task, "Expected to disconnect session")
}

func TestHandlePubSub(t *testing.T) {
	node := NewMockNode()

	go node.hub.Run()
	defer node.hub.Shutdown()

	session := NewMockSession("14", &node)
	session2 := NewMockSession("15", &node)

	node.hub.addSession(session)
	node.hub.subscribeSession("14", "test", "test_channel")

	node.hub.addSession(session2)
	node.hub.subscribeSession("15", "test", "test_channel")

	node.HandlePubSub([]byte("{\"stream\":\"test\",\"data\":\"\\\"abc123\\\"\"}"))

	expected := "{\"identifier\":\"test_channel\",\"message\":\"abc123\"}"

	msg, err := session.conn.Read()
	assert.Nil(t, err)
	assert.Equalf(t, expected, string(msg), "Expected to receive %s but got %s", expected, string(msg))

	msg2, err := session2.conn.Read()
	assert.Nil(t, err)
	assert.Equalf(t, expected, string(msg2), "Expected to receive %s but got %s", expected, string(msg2))
}

func TestHandlePubSubWithCommand(t *testing.T) {
	node := NewMockNode()

	go node.hub.Run()
	defer node.hub.Shutdown()

	session := NewMockSession("14", &node)
	node.hub.addSession(session)

	node.HandlePubSub([]byte("{\"command\":\"disconnect\",\"payload\":{\"identifier\":\"14\",\"reconnect\":false}}"))

	expected := string(toJSON(newDisconnectMessage("remote", false)))

	msg, err := session.conn.Read()
	assert.Nil(t, err)
	assert.Equalf(t, expected, string(msg), "Expected to receive %s but got %s", expected, string(msg))
	assert.True(t, session.closed)
}

func TestSubscriptionsList(t *testing.T) {
	t.Run("with different channels", func(t *testing.T) {
		subscriptions := map[string]bool{
			"{\"channel\":\"SystemNotificationChannel\"}":              true,
			"{\"channel\":\"ClassSectionTableChannel\",\"id\":288528}": true,
			"{\"channel\":\"ScheduleChannel\",\"id\":23376}":           true,
			"{\"channel\":\"DressageChannel\",\"id\":23376}":           true,
			"{\"channel\":\"TimekeepingChannel\",\"id\":23376}":        true,
			"{\"channel\":\"ClassSectionChannel\",\"id\":288528}":      true,
		}

		expected := []string{
			"{\"channel\":\"SystemNotificationChannel\"}",
			"{\"channel\":\"ClassSectionTableChannel\",\"id\":288528}",
			"{\"channel\":\"ScheduleChannel\",\"id\":23376}",
			"{\"channel\":\"DressageChannel\",\"id\":23376}",
			"{\"channel\":\"TimekeepingChannel\",\"id\":23376}",
			"{\"channel\":\"ClassSectionChannel\",\"id\":288528}",
		}

		actual := subscriptionsList(subscriptions)

		for _, key := range expected {
			assert.Contains(t, actual, key)
		}
	})

	t.Run("with the same channel", func(t *testing.T) {
		subscriptions := map[string]bool{
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8949069\"}": true,
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8941e62\"}": true,
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8942db1\"}": true,
		}

		expected := []string{
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8949069\"}",
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8941e62\"}",
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8942db1\"}",
		}

		actual := subscriptionsList(subscriptions)

		for _, key := range expected {
			assert.Contains(t, actual, key)
		}
	})
}
