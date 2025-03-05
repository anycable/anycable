package hub

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockSession struct {
	sid      string
	incoming chan ([]byte)
	closed   bool
	closeMu  sync.Mutex
}

func (s *MockSession) GetID() string {
	return s.sid
}

func (s *MockSession) GetIdentifiers() string {
	return s.sid
}

func (s *MockSession) Send(msg encoders.EncodedMessage) {
	s.incoming <- toJSON(msg)
}

func (s *MockSession) DisconnectWithMessage(msg encoders.EncodedMessage, code string) {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return
	}

	s.incoming <- toJSON(msg)
	s.closed = true
}

func (s *MockSession) Closed() bool {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	return s.closed
}

func (s *MockSession) Read() ([]byte, error) {
	timer := time.After(100 * time.Millisecond)

	select {
	case <-timer:
		return nil, errors.New("Session hasn't received any messages")
	case msg := <-s.incoming:
		return msg, nil
	}
}

func (s *MockSession) ReadIndifinitely() ([]byte, error) {
	return <-s.incoming, nil
}

func NewMockSession(sid string) *MockSession {
	return &MockSession{sid: sid, incoming: make(chan []byte, 256)}
}

func TestUnsubscribeRaceConditions(t *testing.T) {
	hub := NewHub(2, slog.Default())

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123")
	session2 := NewMockSession("321")
	session3 := NewMockSession("213")

	hub.AddSession(session)
	hub.SubscribeSession(session, "test", "test_channel")

	hub.AddSession(session2)
	hub.SubscribeSession(session2, "test", "test_channel")

	hub.AddSession(session3)
	hub.SubscribeSession(session3, "test", "test_channel")

	hub.Broadcast("test", "hello")

	_, err := session.Read()
	assert.Nil(t, err)

	_, err = session2.Read()
	assert.Nil(t, err)

	_, err = session3.Read()
	assert.Nil(t, err)

	assert.Equal(t, 3, hub.Size(), "Connections size must be equal 2")

	go func() {
		hub.Broadcast("test", "pong")
		hub.RemoveSession(session)
		hub.Broadcast("test", "ping")
	}()

	go func() {
		hub.Broadcast("test", "bye-bye")
		hub.RemoveSession(session3)
		hub.Broadcast("test", "meow-meow")
	}()

	for i := 1; i < 5; i++ {
		_, err = session2.Read()
		assert.Nil(t, err)
	}

	_, err = session2.Read()
	assert.NotNil(t, err)

	assert.Equal(t, 1, hub.Size(), "Connections size must be equal 1")
}

func TestUnsubscribeSession(t *testing.T) {
	hub := NewHub(2, slog.Default())

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123")
	hub.AddSession(session)

	hub.SubscribeSession(session, "test", "test_channel")
	hub.SubscribeSession(session, "test2", "test_channel")

	hub.Broadcast("test", "\"hello\"")

	msg, err := session.Read()
	assert.Nil(t, err)
	assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"hello\"}", string(msg))

	hub.UnsubscribeSession(session, "test", "test_channel")

	hub.Broadcast("test", "\"goodbye\"")

	_, err = session.Read()
	assert.NotNil(t, err)

	hub.Broadcast("test2", "\"bye\"")

	msg, err = session.Read()
	assert.Nil(t, err)
	assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"bye\"}", string(msg))

	hub.unsubscribeSessionFromAllChannels(session)

	hub.Broadcast("test2", "\"goodbye\"")

	_, err = session.Read()
	assert.NotNil(t, err)
}

func TestUnsubscribeSessionFromChannel(t *testing.T) {
	hub := NewHub(2, slog.Default())

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123")
	hub.AddSession(session)

	hub.SubscribeSession(session, "test1", "test_channel")
	hub.SubscribeSession(session, "test2", "test_channel")
	hub.SubscribeSession(session, "test3", "other_channel")

	hub.Broadcast("test1", "\"hello1\"")
	msg, err := session.Read()
	assert.Nil(t, err)
	assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"hello1\"}", string(msg))

	hub.Broadcast("test2", "\"hello2\"")
	msg, err = session.Read()
	assert.Nil(t, err)
	assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"hello2\"}", string(msg))

	hub.Broadcast("test3", "\"hello3\"")
	msg, err = session.Read()
	assert.Nil(t, err)
	assert.Equal(t, "{\"identifier\":\"other_channel\",\"message\":\"hello3\"}", string(msg))

	hub.UnsubscribeSessionFromChannel(session, "test_channel")

	hub.Broadcast("test1", "\"goodbye1\"")
	_, err = session.Read()
	assert.NotNil(
		t,
		err,
		"Should not receive message from test1 after unsubscribing from test_channel",
	)

	hub.Broadcast("test2", "\"goodbye2\"")
	_, err = session.Read()
	assert.NotNil(
		t,
		err,
		"Should not receive message from test2 after unsubscribing from test_channel",
	)

	hub.Broadcast("test3", "\"still_here\"")
	msg, err = session.Read()
	assert.Nil(t, err)
	assert.Equal(t, "{\"identifier\":\"other_channel\",\"message\":\"still_here\"}", string(msg))
}

func TestSubscribeSession(t *testing.T) {
	hub := NewHub(2, slog.Default())

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123")
	hub.AddSession(session)

	t.Run("Subscribe to a single channel", func(t *testing.T) {
		hub.SubscribeSession(session, "test", "test_channel")

		hub.Broadcast("test", "\"hello\"")

		msg, err := session.Read()
		assert.Nil(t, err)
		assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"hello\"}", string(msg))
	})

	t.Run("Successful to the same stream from multiple channels", func(t *testing.T) {
		hub.SubscribeSession(session, "test", "test_channel")
		hub.SubscribeSession(session, "test", "test_channel2")

		hub.Broadcast("test", "\"hello twice\"")

		received := []string{}

		msg, err := session.Read()
		assert.Nil(t, err)
		received = append(received, string(msg))

		msg, err = session.Read()
		assert.Nil(t, err)
		received = append(received, string(msg))

		assert.Contains(t, received, "{\"identifier\":\"test_channel\",\"message\":\"hello twice\"}")
		assert.Contains(t, received, "{\"identifier\":\"test_channel2\",\"message\":\"hello twice\"}")
	})
}

func TestRemoteDisconnect(t *testing.T) {
	hub := NewHub(2, slog.Default())

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123")
	hub.AddSession(session)

	t.Run("Disconnect session", func(t *testing.T) {
		hub.RemoteDisconnect(&common.RemoteDisconnectMessage{Identifier: "123", Reconnect: false})

		msg, err := session.Read()
		assert.Nil(t, err)
		assert.Equal(t, "{\"type\":\"disconnect\",\"reason\":\"remote\",\"reconnect\":false}", string(msg))

		assert.True(t, session.Closed())
	})
}

func TestBroadcastMessage(t *testing.T) {
	hub := NewHub(2, slog.Default())

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123")
	hub.AddSession(session)
	hub.SubscribeSession(session, "test", "test_channel")

	t.Run("Broadcast without stream data", func(t *testing.T) {
		hub.BroadcastMessage(&common.StreamMessage{Stream: "test", Data: "\"ciao\""})

		msg, err := session.Read()
		assert.Nil(t, err)
		assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"ciao\"}", string(msg))
	})

	t.Run("Broadcast with stream data", func(t *testing.T) {
		hub.BroadcastMessage(&common.StreamMessage{Stream: "test", Data: "\"ciao\"", Epoch: "xyz", Offset: 2022})

		msg, err := session.Read()
		assert.Nil(t, err)
		assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"ciao\",\"stream_id\":\"test\",\"epoch\":\"xyz\",\"offset\":2022}", string(msg))
	})

	t.Run("Broadcast with exclude_socket", func(t *testing.T) {
		session2 := NewMockSession("234")
		hub.AddSession(session2)
		hub.SubscribeSession(session2, "test", "test_channel")

		hub.BroadcastMessage(&common.StreamMessage{Stream: "test", Data: "\"ciao\""})

		msg, err := session.Read()
		assert.Nil(t, err)
		assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"ciao\"}", string(msg))

		msg, err = session2.Read()
		assert.Nil(t, err)
		assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"ciao\"}", string(msg))

		hub.BroadcastMessage(&common.StreamMessage{
			Stream: "test",
			Data:   "\"hoi!\"",
			Meta: &common.StreamMessageMetadata{
				ExcludeSocket: "234",
			},
		})

		msg, err = session.Read()
		assert.Nil(t, err)
		assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"hoi!\"}", string(msg))

		msg, err = session2.Read()
		assert.Nil(t, msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "hasn't received any messages")
	})
}

func TestBroadcastOrder(t *testing.T) {
	hub := NewHub(10, slog.Default())

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123")
	hub.AddSession(session)
	hub.SubscribeSession(session, "test", "test_channel")

	session2 := NewMockSession("321")
	hub.AddSession(session2)
	hub.SubscribeSession(session2, "test2", "test2_channel")

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		for i := 1; i <= 5; i++ {
			hub.BroadcastMessage(&common.StreamMessage{Stream: "test", Data: fmt.Sprintf("%d", i)})
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 1; i <= 5; i++ {
			hub.BroadcastMessage(&common.StreamMessage{Stream: "test2", Data: fmt.Sprintf("%d", i)})
		}
		wg.Done()
	}()

	wg.Wait()

	for i := 1; i <= 5; i++ {
		msg, err := session.Read()
		assert.Nil(t, err)
		assert.Equal(t, fmt.Sprintf("{\"identifier\":\"test_channel\",\"message\":%d}", i), string(msg))
	}

	for i := 1; i <= 5; i++ {
		msg, err := session2.Read()
		assert.Nil(t, err)
		assert.Equal(t, fmt.Sprintf("{\"identifier\":\"test2_channel\",\"message\":%d}", i), string(msg))
	}
}

func TestBuildMessageJSON(t *testing.T) {
	expected := []byte("{\"identifier\":\"chat\",\"message\":{\"text\":\"hello!\"}}")
	actual := toJSON(buildMessage(&common.StreamMessage{Data: "{\"text\":\"hello!\"}"}, "chat"))
	assert.Equal(t, expected, actual)
}

func TestBuildMessageString(t *testing.T) {
	expected := []byte("{\"identifier\":\"chat\",\"message\":\"plain string\"}")
	actual := toJSON(buildMessage(&common.StreamMessage{Data: "\"plain string\""}, "chat"))
	assert.Equal(t, expected, actual)
}

type benchmarkConfig struct {
	hubPoolSize       int
	totalStreams      int
	totalSessions     int
	streamsPerSession int
	payload           string
}

func BenchmarkBroadcast(b *testing.B) {
	configs := []benchmarkConfig{}

	poolSizes := []int{128, 16, 2, 1}
	streamNums := [][]int{
		{1000, 10},
		{100, 10},
		{10, 3},
	}
	sessionsNum := 10000
	payload := "\"A quick brow fox bla-bla-bla\""

	for _, streamNum := range streamNums {
		for _, poolSize := range poolSizes {
			configs = append(configs, benchmarkConfig{poolSize, streamNum[0], sessionsNum, streamNum[1], payload})
		}
	}

	for _, config := range configs {
		b.Run(fmt.Sprintf("%v", config), func(b *testing.B) {
			broadcastsPerStream := b.N / config.totalStreams
			messagesPerSession := config.streamsPerSession * broadcastsPerStream

			hub := NewHub(config.hubPoolSize, slog.Default())

			go hub.Run()
			defer hub.Shutdown()

			var wg sync.WaitGroup
			var streams []string

			for i := 0; i < config.totalStreams; i++ {
				stream := fmt.Sprintf("stream_%d", i)
				streams = append(streams, stream)
			}

			for i := 0; i < config.totalSessions; i++ {
				sid := fmt.Sprintf("%d", i)
				session := NewMockSession(sid)

				wg.Add(1)

				go func() {
					countdown := 0
					for {
						if countdown >= messagesPerSession {
							wg.Done()
							break
						}

						session.ReadIndifinitely() // nolint:errcheck
						countdown++
					}
				}()

				hub.AddSession(session)

				for j := 0; j < config.streamsPerSession; j++ {
					channel := fmt.Sprintf("test_channel%d", j)

					stream := streams[rand.Intn(len(streams))] // nolint:gosec

					hub.SubscribeSession(session, stream, channel)
				}
			}

			b.ResetTimer()

			for _, stream := range streams {
				for i := 0; i < broadcastsPerStream; i++ {
					hub.Broadcast(stream, config.payload)
				}
			}

			wg.Wait()
			b.StopTimer()
		})
	}
}

func toJSON(msg encoders.EncodedMessage) []byte {
	b, err := json.Marshal(&msg)
	if err != nil {
		panic("Failed to build JSON 😲")
	}

	return b
}
