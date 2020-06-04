package node

import (
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
)

func TestUnsubscribeRaceConditions(t *testing.T) {
	hub := NewHub()

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123")
	session2 := NewMockSession("321")

	hub.addSession(session)
	hub.subscribeSession("123", "test", "test_channel")

	hub.addSession(session2)
	hub.subscribeSession("321", "test", "test_channel")

	hub.broadcast <- &common.StreamMessage{Stream: "test", Data: "hello"}

	<-session.send
	<-session2.send

	assert.Equal(t, 2, hub.Size(), "Connections size must be equal 2")

	go func() {
		hub.broadcast <- &common.StreamMessage{Stream: "test", Data: "pong"}
		hub.unregister <- session
		hub.broadcast <- &common.StreamMessage{Stream: "test", Data: "ping"}
	}()

	go func() {
		hub.broadcast <- &common.StreamMessage{Stream: "test", Data: "bye-bye"}
	}()

	<-session2.send
	<-session2.send
	<-session2.send

	assert.Equal(t, 1, hub.Size(), "Connections size must be equal 1")
}

func TestBuildMessageJSON(t *testing.T) {
	expected := []byte("{\"identifier\":\"chat\",\"message\":{\"text\":\"hello!\"}}")
	actual := buildMessage("{\"text\":\"hello!\"}", "chat")
	assert.Equal(t, expected, actual)
}

func TestBuildMessageString(t *testing.T) {
	expected := []byte("{\"identifier\":\"chat\",\"message\":\"plain string\"}")
	actual := buildMessage("\"plain string\"", "chat")
	assert.Equal(t, expected, actual)
}
