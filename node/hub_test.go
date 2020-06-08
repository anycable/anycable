package node

import (
	"testing"
	"time"

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

func TestUnsubscribeSession(t *testing.T) {
	hub := NewHub()

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123")
	hub.addSession(session)

	hub.subscribeSession("123", "test", "test_channel")
	hub.subscribeSession("123", "test2", "test_channel")

	hub.broadcast <- &common.StreamMessage{Stream: "test", Data: "\"hello\""}

	timer := time.After(100 * time.Millisecond)
	select {
	case <-timer:
		t.Fatalf("Session hasn't received any messages")
	case msg := <-session.send:
		assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"hello\"}", string(msg.payload))
	}

	hub.unsubscribeSession("123", "test", "test_channel")

	hub.broadcast <- &common.StreamMessage{Stream: "test", Data: "\"goodbye\""}

	timer = time.After(100 * time.Millisecond)
	select {
	case <-timer:
	case msg := <-session.send:
		t.Fatalf("Session shouldn't have received any messages but received: %v", string(msg.payload))
	}

	hub.broadcast <- &common.StreamMessage{Stream: "test2", Data: "\"bye\""}

	timer = time.After(100 * time.Millisecond)
	select {
	case <-timer:
		t.Fatalf("Session hasn't received any messages")
	case msg := <-session.send:
		assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"bye\"}", string(msg.payload))
	}

	hub.unsubscribeSessionFromAllChannels("123")

	hub.broadcast <- &common.StreamMessage{Stream: "test2", Data: "\"goodbye\""}

	timer = time.After(100 * time.Millisecond)
	select {
	case <-timer:
	case msg := <-session.send:
		t.Fatalf("Session shouldn't have received any messages but received: %v", string(msg.payload))
	}
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
