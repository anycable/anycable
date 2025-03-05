package node

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestCacheEntry(t *testing.T) {
	session := Session{}

	session.subscriptions = NewSubscriptionState()
	session.subscriptions.AddChannel("chat_1")
	session.subscriptions.AddChannel("presence_1")

	session.subscriptions.AddChannelStream("chat_1", "a")
	session.subscriptions.AddChannelStream("chat_1", "b")
	session.subscriptions.AddChannelStream("presence_1", "z")

	session.env = common.NewSessionEnv("/cable", nil)
	session.SetIdentifiers("plastilin")
	session.env.MergeConnectionState(&map[string]string{"tenant": "x", "locale": "it"})
	session.env.MergeChannelState("chat_1", &map[string]string{"presence": "on"})

	session.MarkDisconnectable(true)

	cached, err := session.ToCacheEntry()
	require.NoError(t, err)

	new_session := Session{}
	new_session.subscriptions = NewSubscriptionState()
	new_session.env = common.NewSessionEnv("/cable", nil)

	err = new_session.RestoreFromCache(cached)
	require.NoError(t, err)

	assert.Equal(t, "plastilin", new_session.GetIdentifiers())

	assert.Contains(t, new_session.subscriptions.Channels(), "chat_1")
	assert.Contains(t, new_session.subscriptions.Channels(), "presence_1")
	assert.Contains(t, new_session.subscriptions.StreamsFor("chat_1"), "a")
	assert.Contains(t, new_session.subscriptions.StreamsFor("chat_1"), "b")
	assert.Contains(t, new_session.subscriptions.StreamsFor("presence_1"), "z")

	assert.Equal(t, "x", new_session.env.GetConnectionStateField("tenant"))
	assert.Equal(t, "it", new_session.env.GetConnectionStateField("locale"))
	assert.Equal(t, "on", new_session.env.GetChannelStateField("chat_1", "presence"))

	assert.True(t, new_session.IsDisconnectable())
}

func TestCacheEntryEmptySession(t *testing.T) {
	session := Session{}
	session.subscriptions = NewSubscriptionState()
	session.env = common.NewSessionEnv("/cable", nil)

	cached, err := session.ToCacheEntry()
	require.NoError(t, err)

	new_session := Session{}
	new_session.subscriptions = NewSubscriptionState()
	new_session.env = common.NewSessionEnv("/cable", nil)

	err = new_session.RestoreFromCache(cached)
	require.NoError(t, err)
}

func TestMarkDisconnectable(t *testing.T) {
	session := Session{}

	session.MarkDisconnectable(false)

	assert.False(t, session.IsDisconnectable())

	session.MarkDisconnectable(true)

	assert.True(t, session.IsDisconnectable())

	session.MarkDisconnectable(false)

	assert.True(t, session.IsDisconnectable())
}

func TestSend__maxPendingSize(t *testing.T) {
	node := NewMockNode()
	session := NewMockSession("123", node)
	session.maxPendingSize = 1

	session.Send(&common.Reply{Type: "message", Message: "hello"})

	msg, err := session.conn.Read()
	assert.Nil(t, err)

	assert.Equal(t, `{"type":"message","message":"hello"}`, string(msg))

	// lock writing
	session.wmu.Lock()

	session.Send(&common.Reply{Type: "message", Message: strings.Repeat("karavana", 64)})
	// Make sure the message is picked up the the writer routine
	time.Sleep(100 * time.Millisecond)

	// this one should be dropped
	session.Send(&common.Reply{Type: "message", Message: strings.Repeat("marivana", 64)})

	// The queue has already > 1kb of data — we must close the session
	session.Send(&common.Reply{Type: "message", Message: strings.Repeat("karambas", 64)})
	session.wmu.Unlock()

	// Read the second message
	msg, err = session.conn.Read()
	assert.Nil(t, err)
	assert.Contains(t, string(msg), "karavana")
	// must receive only slow disconnect notice
	msg, err = session.conn.Read()
	assert.Nil(t, err)
	assert.Equal(t, `{"type":"disconnect","reason":"too_slow","reconnect":true}`, string(msg))
}
