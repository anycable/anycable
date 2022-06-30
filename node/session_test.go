package node

import (
	"sync"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"
	"github.com/stretchr/testify/assert"
)

func TestSendRaceConditions(t *testing.T) {
	node := NewMockNode()
	var wg sync.WaitGroup

	for i := 1; i <= 10; i++ {
		session := NewMockSession("123", node)

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
	session := NewMockSession("123", node)

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
	session := NewMockSession("123", node)
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

func TestMergeEnv(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("123", node)

	istate := map[string]map[string]string{
		"test_channel": {
			"foo": "bar",
			"a":   "z",
		},
	}
	cstate := map[string]string{"_s_": "id=42"}
	origEnv := common.SessionEnv{ChannelStates: &istate, ConnectionState: &cstate}

	session.SetEnv(&origEnv)

	istate2 := map[string]map[string]string{
		"test_channel": {
			"foo": "baz",
		},
		"another_channel": {
			"wasting": "time",
		},
	}

	env := common.SessionEnv{ChannelStates: &istate2}

	cstate2 := map[string]string{"red": "end of silence"}

	env2 := common.SessionEnv{ConnectionState: &cstate2}

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		session.MergeEnv(&env)
		wg.Done()
	}()

	go func() {
		session.MergeEnv(&env2)
		wg.Done()
	}()

	wg.Wait()

	assert.Equal(t, &origEnv, session.GetEnv())

	assert.Equal(t, "id=42", origEnv.GetConnectionStateField("_s_"))
	assert.Equal(t, "end of silence", origEnv.GetConnectionStateField("red"))

	assert.Equal(t, "baz", origEnv.GetChannelStateField("test_channel", "foo"))
	assert.Equal(t, "z", origEnv.GetChannelStateField("test_channel", "a"))
	assert.Equal(t, "time", origEnv.GetChannelStateField("another_channel", "wasting"))
}
