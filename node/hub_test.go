package node

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnsubscribeRaceConditions(t *testing.T) {
	hub := NewHub(2)
	node := NewMockNode()

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123", &node)
	session2 := NewMockSession("321", &node)
	session3 := NewMockSession("213", &node)

	hub.addSession(session)
	hub.subscribeSession("123", "test", "test_channel")

	hub.addSession(session2)
	hub.subscribeSession("321", "test", "test_channel")

	hub.addSession(session3)
	hub.subscribeSession("213", "test", "test_channel")

	hub.Broadcast("test", "hello")

	_, err := session.conn.Read()
	assert.Nil(t, err)

	_, err = session2.conn.Read()
	assert.Nil(t, err)

	_, err = session3.conn.Read()
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
		_, err = session2.conn.Read()
		assert.Nil(t, err)
	}

	_, err = session2.conn.Read()
	assert.NotNil(t, err)

	assert.Equal(t, 1, hub.Size(), "Connections size must be equal 1")
}

func TestUnsubscribeSession(t *testing.T) {
	hub := NewHub(2)
	node := NewMockNode()

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123", &node)
	hub.addSession(session)

	hub.subscribeSession("123", "test", "test_channel")
	hub.subscribeSession("123", "test2", "test_channel")

	hub.Broadcast("test", "\"hello\"")

	msg, err := session.conn.Read()
	assert.Nil(t, err)
	assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"hello\"}", string(msg))

	hub.unsubscribeSession("123", "test", "test_channel")

	hub.Broadcast("test", "\"goodbye\"")

	_, err = session.conn.Read()
	assert.NotNil(t, err)

	hub.Broadcast("test2", "\"bye\"")

	msg, err = session.conn.Read()
	assert.Nil(t, err)
	assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"bye\"}", string(msg))

	hub.unsubscribeSessionFromAllChannels("123")

	hub.Broadcast("test2", "\"goodbye\"")

	_, err = session.conn.Read()
	assert.NotNil(t, err)
}

func TestSubscribeSession(t *testing.T) {
	hub := NewHub(2)
	node := NewMockNode()

	go hub.Run()
	defer hub.Shutdown()

	session := NewMockSession("123", &node)
	hub.addSession(session)

	t.Run("Subscribe to a single channel", func(t *testing.T) {
		hub.subscribeSession("123", "test", "test_channel")

		hub.Broadcast("test", "\"hello\"")

		msg, err := session.conn.Read()
		assert.Nil(t, err)
		assert.Equal(t, "{\"identifier\":\"test_channel\",\"message\":\"hello\"}", string(msg))
	})

	t.Run("Successful to the same stream from multiple channels", func(t *testing.T) {
		hub.subscribeSession("123", "test", "test_channel")
		hub.subscribeSession("123", "test", "test_channel2")

		hub.Broadcast("test", "\"hello twice\"")

		received := []string{}

		msg, err := session.conn.Read()
		assert.Nil(t, err)
		received = append(received, string(msg))

		msg, err = session.conn.Read()
		assert.Nil(t, err)
		received = append(received, string(msg))

		assert.Contains(t, received, "{\"identifier\":\"test_channel\",\"message\":\"hello twice\"}")
		assert.Contains(t, received, "{\"identifier\":\"test_channel2\",\"message\":\"hello twice\"}")
	})
}

func TestBuildMessageJSON(t *testing.T) {
	expected := []byte("{\"identifier\":\"chat\",\"message\":{\"text\":\"hello!\"}}")
	actual := buildMessage("{\"text\":\"hello!\"}", "chat").ToJSON()
	assert.Equal(t, expected, actual)
}

func TestBuildMessageString(t *testing.T) {
	expected := []byte("{\"identifier\":\"chat\",\"message\":\"plain string\"}")
	actual := buildMessage("\"plain string\"", "chat").ToJSON()
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

			hub := NewHub(config.hubPoolSize)
			node := NewMockNode()

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
				session := NewMockSession(sid, &node)

				rand.Seed(time.Now().Unix())

				wg.Add(1)

				go func() {
					countdown := 0
					for {
						if countdown >= messagesPerSession {
							wg.Done()
							break
						}

						(session.conn.(MockConnection)).ReadIndifinitely()
						countdown++
					}
				}()

				hub.addSession(session)

				for j := 0; j < config.streamsPerSession; j++ {
					channel := fmt.Sprintf("test_channel%d", j)

					stream := streams[rand.Intn(len(streams))]

					hub.subscribeSession(sid, stream, channel)
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
