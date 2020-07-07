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

	done := make(chan bool)
	timer := time.After(100 * time.Millisecond)

	go func() {
		<-session.send
		<-session2.send
		done <- true
	}()

	select {
	case <-timer:
		t.Fatalf("Session hasn't received any messages")
	case <-done:
	}

	assert.Equal(t, 2, hub.Size(), "Connections size must be equal 2")

	go func() {
		hub.broadcast <- &common.StreamMessage{Stream: "test", Data: "pong"}
		hub.unregister <- session
		hub.broadcast <- &common.StreamMessage{Stream: "test", Data: "ping"}
	}()

	go func() {
		hub.broadcast <- &common.StreamMessage{Stream: "test", Data: "bye-bye"}
	}()

	timer2 := time.After(2500 * time.Millisecond)
	go func() {
		<-session2.send
		<-session2.send
		<-session2.send
		<-session.send
		done <- true
	}()

	select {
	case <-timer2:
		t.Fatalf("Session hasn't received any messages")
	case <-done:
	}

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

func TestSubscribeSession(t *testing.T) {
	hub := NewHub()

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123")
	hub.addSession(session)

	t.Run("Subscribe to a single channel", func(t *testing.T) {
		hub.subscribeSession("123", "test", "test_channel")

		hub.broadcast <- &common.StreamMessage{Stream: "test", Data: "\"hello\""}

		timer := time.After(100 * time.Millisecond)
		select {
		case <-timer:
			t.Fatalf("Session hasn't received any messages")
		case msg := <-session.send:
			assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"hello\"}", string(msg.payload))
		}
	})

	t.Run("Successful to the same stream from multiple channels", func(t *testing.T) {
		hub.subscribeSession("123", "test", "test_channel")
		hub.subscribeSession("123", "test", "test_channel2")

		hub.broadcast <- &common.StreamMessage{Stream: "test", Data: "\"hello twice\""}

		done := make(chan bool)
		received := []string{}

		go func() {
			received = append(received, string((<-session.send).payload))
			received = append(received, string((<-session.send).payload))
			done <- true
		}()

		timer := time.After(100 * time.Millisecond)
		select {
		case <-timer:
			t.Fatalf("Session hasn't received enough messages. Received: %v", received)
		case <-done:
			assert.Contains(t, received, "{\"identifier\":\"test_channel\",\"message\":\"hello twice\"}")
			assert.Contains(t, received, "{\"identifier\":\"test_channel2\",\"message\":\"hello twice\"}")
		}
	})
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
