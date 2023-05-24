package node

import (
	"sync"
	"testing"

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

func TestSubscriptionStateChannels(t *testing.T) {
	t.Run("with different channels", func(t *testing.T) {
		subscriptions := NewSubscriptionState()

		subscriptions.AddChannel("{\"channel\":\"SystemNotificationChannel\"}")
		subscriptions.AddChannel("{\"channel\":\"DressageChannel\",\"id\":23376}")

		expected := []string{
			"{\"channel\":\"SystemNotificationChannel\"}",
			"{\"channel\":\"DressageChannel\",\"id\":23376}",
		}

		actual := subscriptions.Channels()

		for _, key := range expected {
			assert.Contains(t, actual, key)
		}
	})

	t.Run("with the same channel", func(t *testing.T) {
		subscriptions := NewSubscriptionState()

		subscriptions.AddChannel(
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8949069\"}",
		)
		subscriptions.AddChannel(
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8941e62\"}",
		)

		expected := []string{
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8949069\"}",
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8941e62\"}",
		}

		actual := subscriptions.Channels()

		for _, key := range expected {
			assert.Contains(t, actual, key)
		}
	})
}

func TestSubscriptionStreamsFor(t *testing.T) {
	subscriptions := NewSubscriptionState()

	subscriptions.AddChannel("chat_1")
	subscriptions.AddChannel("presence_1")

	subscriptions.AddChannelStream("chat_1", "a")
	subscriptions.AddChannelStream("chat_1", "b")
	subscriptions.AddChannelStream("presence_1", "z")

	assert.Contains(t, subscriptions.StreamsFor("chat_1"), "a")
	assert.Contains(t, subscriptions.StreamsFor("chat_1"), "b")
	assert.Equal(t, []string{"z"}, subscriptions.StreamsFor("presence_1"))

	subscriptions.RemoveChannelStreams("chat_1")
	assert.Empty(t, subscriptions.StreamsFor("chat_1"))
	assert.Equal(t, []string{"z"}, subscriptions.StreamsFor("presence_1"))

	subscriptions.AddChannelStream("presence_1", "y")
	subscriptions.RemoveChannelStream("presence_1", "z")
	subscriptions.RemoveChannelStream("presence_1", "t")
	assert.Equal(t, []string{"y"}, subscriptions.StreamsFor("presence_1"))
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

func TestPrevSid(t *testing.T) {
	session := Session{}
	headers := make(map[string]string)
	session.env = common.NewSessionEnv("ws://example.dev/cable", nil)
	assert.Equal(t, "", session.PrevSid())

	session.env = common.NewSessionEnv("ws://example.dev/cable?sid=123", &headers)
	assert.Equal(t, "123", session.PrevSid())

	session.env = common.NewSessionEnv("http://example.dev/cable?jid=xxxx&sid=213", &headers)
	assert.Equal(t, "213", session.PrevSid())

	headers["X-ANYCABLE-RESTORE-SID"] = "456"

	session.env = common.NewSessionEnv("http://example.dev/cable?jid=xxxx&sid=213", &headers)
	assert.Equal(t, "456", session.PrevSid())
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
