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
