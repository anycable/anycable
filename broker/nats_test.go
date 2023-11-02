package broker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/enats"
	natsconfig "github.com/anycable/anycable-go/nats"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FakeBroadastHandler struct {
}

func (FakeBroadastHandler) Broadcast(msg *common.StreamMessage) {
}

func (FakeBroadastHandler) ExecuteRemoteCommand(cmd *common.RemoteCommandMessage) {
}

var _ pubsub.Handler = (*FakeBroadastHandler)(nil)

func TestNATSBroker_HistorySince_expiration(t *testing.T) {
	port := 32
	addr := fmt.Sprintf("nats://127.0.0.1:44%d", port)
	server := buildNATSServer(t, addr)
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := NewConfig()
	config.HistoryTTL = 1

	nconfig := natsconfig.NewNATSConfig()
	nconfig.Servers = addr

	broadcastHandler := FakeBroadastHandler{}
	broadcaster := pubsub.NewLegacySubscriber(broadcastHandler)
	broker := NewNATSBroker(broadcaster, &config, &nconfig)

	err = broker.Start()
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck

	// Ensure no stream exists
	require.NoError(t, broker.Reset())

	// We must subscribe to receive messages from the stream
	broker.Subscribe("test")
	defer broker.Unsubscribe("test")

	start := time.Now().Unix() - 10

	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "a"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "b"})

	// Stream must be expired after 1 second
	time.Sleep(2 * time.Second)

	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "c"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "d"})

	history, err := broker.HistorySince("test", start)
	require.NoError(t, err)

	require.Len(t, history, 2)
	assert.EqualValues(t, 3, history[0].Offset)
	assert.Equal(t, "c", history[0].Data)
	assert.EqualValues(t, 4, history[1].Offset)
	assert.Equal(t, "d", history[1].Data)

	// Stream must be expired after 1 second
	time.Sleep(2 * time.Second)

	history, err = broker.HistorySince("test", start)
	require.NoError(t, err)
	assert.Empty(t, history)
}

func TestNATSBroker_HistorySince_with_limit(t *testing.T) {
	port := 33
	addr := fmt.Sprintf("nats://127.0.0.1:44%d", port)
	server := buildNATSServer(t, addr)
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := NewConfig()
	config.HistoryLimit = 2

	nconfig := natsconfig.NewNATSConfig()
	nconfig.Servers = addr

	broadcastHandler := FakeBroadastHandler{}
	broadcaster := pubsub.NewLegacySubscriber(broadcastHandler)
	broker := NewNATSBroker(broadcaster, &config, &nconfig)

	err = broker.Start()
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck

	// Ensure no stream exists
	require.NoError(t, broker.Reset())

	// We must subscribe to receive messages from the stream
	broker.Subscribe("test")
	defer broker.Unsubscribe("test")

	start := time.Now().Unix() - 10

	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "a"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "b"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "c"})

	history, err := broker.HistorySince("test", start)
	require.NoError(t, err)

	assert.Len(t, history, 2)
	assert.EqualValues(t, 3, history[1].Offset)
	assert.Equal(t, "c", history[1].Data)
}

func TestNATSBroker_HistoryFrom(t *testing.T) {
	port := 34
	addr := fmt.Sprintf("nats://127.0.0.1:44%d", port)
	server := buildNATSServer(t, addr)

	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := NewConfig()

	nconfig := natsconfig.NewNATSConfig()
	nconfig.Servers = addr

	broadcastHandler := FakeBroadastHandler{}
	broadcaster := pubsub.NewLegacySubscriber(broadcastHandler)
	broker := NewNATSBroker(broadcaster, &config, &nconfig)

	err = broker.Start()
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck

	// Ensure no stream exists
	require.NoError(t, broker.Reset())

	// We must subscribe to receive messages from the stream
	broker.Subscribe("test")
	defer broker.Unsubscribe("test")

	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "y"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "z"})

	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "a"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "b"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "c"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "d"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "e"})

	// We obtain sequence numbers with some offset to ensure that we use sequence numbers
	// as stream offsets
	seq := consumerSequence(broker.js, "test", 3)

	offsets, err := seq.read(5)
	require.NoError(t, err)

	t.Run("With current epoch", func(t *testing.T) {
		require.EqualValues(t, 4, offsets[1])

		history, err := broker.HistoryFrom("test", broker.Epoch(), offsets[1])
		require.NoError(t, err)

		assert.Len(t, history, 3)
		assert.EqualValues(t, 5, history[0].Offset)
		assert.Equal(t, "c", history[0].Data)
		assert.EqualValues(t, 6, history[1].Offset)
		assert.Equal(t, "d", history[1].Data)
		assert.EqualValues(t, 7, history[2].Offset)
		assert.Equal(t, "e", history[2].Data)
	})

	t.Run("When no new messages", func(t *testing.T) {
		history, err := broker.HistoryFrom("test", broker.Epoch(), offsets[4])
		require.NoError(t, err)
		assert.Len(t, history, 0)
	})

	t.Run("When no stream", func(t *testing.T) {
		history, err := broker.HistoryFrom("unknown", broker.Epoch(), offsets[1])
		require.Error(t, err)
		assert.Nil(t, history)
	})

	t.Run("With unknown epoch", func(t *testing.T) {
		history, err := broker.HistoryFrom("test", "unknown", offsets[1])
		require.Error(t, err)
		require.Nil(t, history)
	})
}

type TestCacheable struct {
	data string
}

func (t *TestCacheable) ToCacheEntry() ([]byte, error) {
	return []byte(t.data), nil
}

func TestNATSBroker_Sessions(t *testing.T) {
	port := 41
	addr := fmt.Sprintf("nats://127.0.0.1:44%d", port)
	server := buildNATSServer(t, addr)
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := NewConfig()
	config.SessionsTTL = 1

	nconfig := natsconfig.NewNATSConfig()
	nconfig.Servers = addr

	broker := NewNATSBroker(nil, &config, &nconfig)

	err = broker.Start()
	require.NoError(t, err)

	defer broker.Shutdown(context.Background()) // nolint: errcheck

	err = broker.CommitSession("test123", &TestCacheable{"cache-me"})
	require.NoError(t, err)

	anotherBroker := NewNATSBroker(nil, &config, &nconfig)
	anotherBroker.Start()                              // nolint: errcheck
	defer anotherBroker.Shutdown(context.Background()) // nolint: errcheck

	restored, err := anotherBroker.RestoreSession("test123")

	require.NoError(t, err)
	assert.Equalf(t, []byte("cache-me"), restored, "Expected to restore session data: %s", restored)

	// Expiration
	time.Sleep(2 * time.Second)

	expired, err := broker.RestoreSession("test123")
	require.NoError(t, err)
	assert.Nil(t, expired)

	err = broker.CommitSession("test345", &TestCacheable{"cache-me-again"})
	require.NoError(t, err)

	err = broker.FinishSession("test345")
	require.NoError(t, err)

	finished, err := anotherBroker.RestoreSession("test345")

	require.NoError(t, err)
	assert.Equal(t, []byte("cache-me-again"), finished)

	// Expiration
	time.Sleep(2 * time.Second)

	finishedStale, err := anotherBroker.RestoreSession("test345")
	require.NoError(t, err)
	assert.Nil(t, finishedStale)
}

func TestNATSBroker_SessionsTTLChange(t *testing.T) {
	port := 43
	addr := fmt.Sprintf("nats://127.0.0.1:44%d", port)

	server := buildNATSServer(t, addr)
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := NewConfig()
	config.SessionsTTL = 1

	nconfig := natsconfig.NewNATSConfig()
	nconfig.Servers = addr

	broker := NewNATSBroker(nil, &config, &nconfig)

	err = broker.Start()
	require.NoError(t, err)

	defer broker.Shutdown(context.Background()) // nolint: errcheck

	err = broker.CommitSession("test123", &TestCacheable{"cache-me"})
	require.NoError(t, err)

	aConfig := NewConfig()
	aConfig.SessionsTTL = 3

	anotherBroker := NewNATSBroker(nil, &aConfig, &nconfig)
	require.NoError(t, anotherBroker.Start())          // nolint: errcheck
	defer anotherBroker.Shutdown(context.Background()) // nolint: errcheck

	// The session must be missing since we recreated the bucket due to TTL change
	missing, err := anotherBroker.RestoreSession("test123")

	require.NoError(t, err)
	assert.Nil(t, missing)

	err = anotherBroker.CommitSession("test234", &TestCacheable{"cache-me-again"})
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	// Shouldn't fail and catch up a new bucket
	restored, err := broker.RestoreSession("test234")
	require.NoError(t, err)
	assert.Equalf(t, []byte("cache-me-again"), restored, "Expected to restore session data: %s", restored)

	// Touch session
	err = anotherBroker.FinishSession("test234")
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	restoredAgain, err := broker.RestoreSession("test234")
	require.NoError(t, err)
	assert.NotNil(t, restoredAgain)

	time.Sleep(1 * time.Second)

	expired, err := broker.RestoreSession("test234")
	require.NoError(t, err)
	assert.Nil(t, expired)
}

func TestNATSBroker_Epoch(t *testing.T) {
	port := 45
	addr := fmt.Sprintf("nats://127.0.0.1:44%d", port)

	server := buildNATSServer(t, addr)
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := NewConfig()

	nconfig := natsconfig.NewNATSConfig()
	nconfig.Servers = addr

	broker := NewNATSBroker(nil, &config, &nconfig)

	err = broker.Start()
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck

	broker.Reset() // nolint: errcheck

	epoch := broker.Epoch()

	anotherBroker := NewNATSBroker(nil, &config, &nconfig)
	require.NoError(t, anotherBroker.Start())          // nolint: errcheck
	defer anotherBroker.Shutdown(context.Background()) // nolint: errcheck

	assert.Equal(t, epoch, anotherBroker.Epoch())

	// Now let's test that epoch changes are picked up
	require.NoError(t, anotherBroker.SetEpoch("new-epoch"))

	assert.Equal(t, "new-epoch", anotherBroker.Epoch())
	assert.Equal(t, "new-epoch", anotherBroker.local.GetEpoch())

	timer := time.After(2 * time.Second)

wait:
	for {
		select {
		case <-timer:
			assert.Fail(t, "Epoch change wasn't picked up")
			return
		default:
			if broker.Epoch() == "new-epoch" {
				break wait
			}

			time.Sleep(100 * time.Millisecond)
		}
	}
}

func buildNATSServer(t *testing.T, addr string) *enats.Service {
	conf := enats.NewConfig()
	conf.JetStream = true
	conf.ServiceAddr = addr
	conf.StoreDir = t.TempDir()
	service := enats.NewService(&conf)

	return service
}

type consumerSequenceReader struct {
	seq chan uint64
}

func (r *consumerSequenceReader) read(n int) ([]uint64, error) {
	seq := make([]uint64, n)
	i := 0

	for {
		select {
		case v := <-r.seq:
			seq[i] = v
			i++
			if i == n {
				return seq, nil
			}
		case <-time.After(1 * time.Second):
			return nil, fmt.Errorf("timed out to read from consumer; received %d messages, expected %d", i, n)
		}
	}
}

func consumerSequence(js jetstream.JetStream, stream string, offset uint64) *consumerSequenceReader {
	seq := make(chan uint64)

	conf := jetstream.ConsumerConfig{
		AckPolicy: jetstream.AckExplicitPolicy,
	}

	if offset > 0 {
		conf.OptStartSeq = offset
		conf.DeliverPolicy = jetstream.DeliverByStartSequencePolicy
	}

	cons, err := js.CreateConsumer(context.Background(), streamPrefix+stream, conf)

	if err != nil {
		panic(err)
	}

	cons.Consume(func(msg jetstream.Msg) { // nolint:errcheck
		meta, _ := msg.Metadata()
		seq <- meta.Sequence.Stream
	})

	return &consumerSequenceReader{seq}
}
