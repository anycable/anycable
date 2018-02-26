package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthenticateSuccess(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("1")

	id, err := node.Authenticate(session, "/cable", &map[string]string{"id": "test_id"})

	assert.Nil(t, err, "Error must be nil")
	assert.Equal(t, "test_id", id, "Must return identifier")
}

func TestAuthenticateFailure(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("1")

	_, err := node.Authenticate(session, "/failure", &map[string]string{"id": "test_id"})

	assert.NotNil(t, err, "Error must not be nil")
}

func TestSubscribeSuccess(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14")

	node.Subscribe(session, &Message{Identifier: "test_channel"})

	// Adds subscription to session
	assert.Contains(t, session.subscriptions, "test_channel", "Session subscription must be set")

	// Expected to send message to session
	var msg []byte

	select {
	case m := <-session.send:
		msg = m
	default:
		assert.Fail(t, "Expected session to receive message but none was sent")
	}

	assert.Equalf(t, []byte("14"), msg, "Sent message is invalid: %s", msg)
}

func TestSubscribeWithStreamSuccess(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14")

	node.Subscribe(session, &Message{Identifier: "stream"})

	// Adds subscription to session
	assert.Contains(t, session.subscriptions, "stream", "Session subsription must be set")

	var subscription SubscriptionInfo

	// Expected to subscribe session to hub
	select {
	case sub := <-node.hub.subscribe:
		subscription = *sub
	default:
		assert.Fail(t, "Expected hub to receive subscribe message but none was sent")
	}

	assert.Equalf(t, "14", subscription.session, "Session is invalid: %s", subscription.session)
	assert.Equalf(t, "stream", subscription.identifier, "Channel is invalid: %s", subscription.identifier)
	assert.Equalf(t, "stream", subscription.stream, "Stream is invalid: %s", subscription.stream)

	// Expected to send message to session
	var msg []byte

	select {
	case m := <-session.send:
		msg = m
	default:
		assert.Fail(t, "Expected session to receive message but none was sent")
	}

	assert.Equalf(t, []byte("14"), msg, "Sent message is invalid: %s", msg)
}

func TestUnsubscribeSuccess(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14")

	session.subscriptions["test_channel"] = true

	node.Unsubscribe(session, &Message{Identifier: "test_channel"})

	// Removes subscription from session
	assert.NotContains(t, session.subscriptions, "test_channel", "Shouldn't contain test_channel")

	var subscription SubscriptionInfo

	// Expected to subscribe session to hub
	select {
	case sub := <-node.hub.unsubscribe:
		subscription = *sub
	default:
		assert.Fail(t, "Expected hub to receive unsubscribe message but none was sent")
	}

	assert.Equalf(t, "14", subscription.session, "Session is invalid: %s", subscription.session)
	assert.Equalf(t, "test_channel", subscription.identifier, "Channel is invalid: %s", subscription.identifier)

	// Expected to send message to session
	var msg []byte

	select {
	case m := <-session.send:
		msg = m
	default:
		assert.Fail(t, "Expected session to receive message but none was sent")
	}

	assert.Equalf(t, []byte("14"), msg, "Sent message is invalid: %s", msg)
}

func TestPerformSuccess(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("14")

	session.subscriptions["test_channel"] = true
	node.Perform(session, &Message{Identifier: "test_channel", Data: "action"})

	// Expected to send message to session
	var msg []byte

	select {
	case m := <-session.send:
		msg = m
	default:
		assert.Fail(t, "Expected session to receive message but none was sent")
	}

	assert.Equalf(t, []byte("action"), msg, "Sent message is invalid: %s", msg)
}
