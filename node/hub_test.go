package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnsubscribeRaceConditions(t *testing.T) {
	hub := NewHub()

	go hub.Run()
	defer hub.Shutdown()

	session := &Session{send: make(chan []byte, 256), UID: "123"}

	go func() {
		hub.register <- session
		hub.subscribe <- &subscriptionInfo{session: "123", stream: "test", identifier: "test_channel"}
		hub.broadcast <- &StreamMessage{Stream: "test", Data: "hello"}
	}()

	<-session.send

	assert.Equal(t, 1, hub.Size(), "Connections size must be equal 1")

	done := make(chan bool)
	defer close(done)

	go func() {
		hub.unregister <- session
		hub.broadcast <- &StreamMessage{Stream: "test", Data: "ping"}
		done <- true
	}()

	go func() {
		hub.broadcast <- &StreamMessage{Stream: "test", Data: "bye-bye"}
		done <- true
	}()

	<-done
	<-done

	assert.Equal(t, 0, hub.Size(), "Connections size must be equal 0")
}

func TestBuildMessageJSON(t *testing.T) {
	expected := []byte("{\"identifier\":\"chat\",\"message\":{\"text\":\"hello!\"}}")
	assert.Equal(t, expected, buildMessage("{\"text\":\"hello!\"}", "chat"))
}

func TestBuildMessageString(t *testing.T) {
	expected := []byte("{\"identifier\":\"chat\",\"message\":\"plain string\"}")
	assert.Equal(t, expected, buildMessage("\"plain string\"", "chat"))
}
