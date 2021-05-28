package node

import (
	"sync"
	"testing"

	"github.com/anycable/anycable-go/ws"
	"github.com/stretchr/testify/assert"
)

func TestSendRaceConditions(t *testing.T) {
	node := NewMockNode()
	var wg sync.WaitGroup

	for i := 1; i <= 10; i++ {
		session := NewMockSession("123", &node)

		go func() {
			for {
				session.conn.Read() // nolint:errcheck
			}
		}()

		wg.Add(2)
		go func() {
			go func() {
				session.sendFrame(&ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte("hi!")})
				wg.Done()
			}()

			go func() {
				session.sendFrame(&ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte("bye")})
				wg.Done()
			}()
		}()

		wg.Add(2)
		go func() {
			go func() {
				session.sendFrame(&ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte("bye")})
				wg.Done()
			}()

			go func() {
				session.sendFrame(&ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte("why")})
				wg.Done()
			}()
		}()
	}

	wg.Wait()
}

func TestSessionSend(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("123", &node)

	go func() {
		for i := 1; i <= 10; i++ {
			session.sendFrame(&ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte("bye")})
		}
	}()

	for i := 1; i <= 10; i++ {
		_, err := session.conn.Read()
		assert.Nil(t, err)
	}
}

func TestSessionDisconnect(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("123", &node)
	session.closed = false
	session.Connected = true

	go func() {
		session.sendFrame(&ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte("bye")})
		session.Disconnect("test", 1042)
	}()

	// Message frame
	_, err := session.conn.Read()
	assert.Nil(t, err)

	// Close frame
	_, err = session.conn.Read()
	assert.Nil(t, err)
}
