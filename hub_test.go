package main

import (
	"testing"
)

func TestUnsubscribeRaceConditions(t *testing.T) {
	hub := &Hub{
		broadcast:   make(chan []byte),
		register:    make(chan *Conn),
		unregister:  make(chan *Conn),
		connections: make(map[*Conn]bool),
		shutdown:    make(chan bool),
	}

	go hub.run()
	defer hub.Shutdown()

	conn := &Conn{send: make(chan []byte)}

	go func() {
		hub.register <- conn
		hub.broadcast <- []byte("hello!")
	}()

	<-conn.send

	if hub.Size() != 1 {
		t.Errorf("Connections size must be equal 1, got %d", hub.Size())
	}

	done := make(chan bool)
	defer close(done)

	go func() {
		hub.unregister <- conn
		hub.broadcast <- []byte("ping")
		done <- true
	}()

	go func() {
		hub.broadcast <- []byte("good-bye!")
		done <- true
	}()

	<-done
	<-done

	if hub.Size() != 0 {
		t.Errorf("Connections size must be equal 0, got %d", hub.Size())
	}
}
