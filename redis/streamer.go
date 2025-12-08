package redis

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/anycable/anycable-go/utils"

	nanoid "github.com/matoous/go-nanoid"
	"github.com/redis/rueidis"
)

type StreamHandler = func(map[string]string) error

// Streamer represents a generic Redis stream consumer
// with auto-reconnect and ack logic
type Streamer struct {
	handler StreamHandler
	config  *RedisConfig

	stream   string
	group    string
	block_ms int64

	// Unique consumer identifier
	consumerName string

	client        rueidis.Client
	clientOptions *rueidis.ClientOption
	clientMu      sync.RWMutex

	reconnectAttempt int

	shutdownCh chan struct{}
	finishedCh chan struct{}

	log *slog.Logger
}

type StreamerOption func(*Streamer)

func StreamerWithBlockMS(ms int64) StreamerOption {
	return func(s *Streamer) {
		s.block_ms = ms
	}
}

func StreamerWithConsumerName(name string) StreamerOption {
	return func(s *Streamer) {
		s.consumerName = name
	}
}

func StreamerWithHandler(h StreamHandler) StreamerOption {
	return func(s *Streamer) {
		s.handler = h
	}
}

// NewStreamer builds a new Streamer
func NewStreamer(stream string, group string, config *RedisConfig, l *slog.Logger, opts ...StreamerOption) *Streamer {
	name, _ := nanoid.Nanoid(6)

	s := &Streamer{
		config:       config,
		consumerName: name,
		stream:       stream,
		group:        group,
		log:          l,
		block_ms:     2000,
		shutdownCh:   make(chan struct{}),
		finishedCh:   make(chan struct{}),
		handler:      func(map[string]string) (err error) { return },
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Streamer) Start() error {
	options, err := s.config.ToRueidisOptions()

	if err != nil {
		return err
	}

	s.clientOptions = options

	go s.runReader()

	return nil
}

func (s *Streamer) Shutdown(ctx context.Context) error {
	s.clientMu.RLock()
	defer s.clientMu.RUnlock()

	if s.client == nil {
		return nil
	}

	s.log.Debug("shutting down Redis streamer")

	close(s.shutdownCh)

	<-s.finishedCh

	res := s.client.Do(
		context.Background(),
		s.client.B().XgroupDelconsumer().Key(s.stream).Group(s.group).Consumername(s.consumerName).Build(),
	)

	err := res.Error()

	if err != nil {
		s.log.Error("failed to remove Redis stream consumer", "error", err)
	}

	s.client.Close()

	return nil
}

func (s *Streamer) initClient() error {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	if s.client != nil {
		return nil
	}

	c, err := rueidis.NewClient(*s.clientOptions)

	if err != nil {
		return err
	}

	s.client = c

	return nil
}

func (s *Streamer) Client() rueidis.Client {
	s.initClient() // nolint:errcheck

	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	return s.client
}

func (s *Streamer) runReader() {
	err := s.initClient()

	if err != nil {
		s.log.Error("failed to connect to Redis", "error", err)
		s.maybeReconnect()
		return
	}

	if s.reconnectAttempt > 0 {
		s.log.Info("reconnected to Redis")
	}

	s.reconnectAttempt = 0

	// First, create a consumer group for the stream
	err = s.client.Do(context.Background(),
		s.client.B().XgroupCreate().Key(s.stream).Group(s.group).Id("$").Mkstream().Build(),
	).Error()

	if err != nil {
		if redisErr, ok := rueidis.IsRedisErr(err); ok {
			if strings.HasPrefix(redisErr.Error(), "BUSYGROUP") {
				s.log.Debug("Redis consumer group already exists")
			} else {
				s.log.Error("failed to create consumer group", "error", err)
				s.maybeReconnect()
				return
			}
		}
	}

	readBlockMilliseconds := s.block_ms
	var lastClaimedAt int64

	for {
		select {
		case <-s.shutdownCh:
			s.log.Debug("stop consuming stream")
			close(s.finishedCh)
			return
		default:
			if lastClaimedAt+readBlockMilliseconds < time.Now().UnixMilli() {
				reclaimed, err := s.autoclaimMessages(readBlockMilliseconds)

				if err != nil {
					s.log.Error("failed to claim from Redis stream", "error", err)
					s.maybeReconnect()
					return
				}

				lastClaimedAt = time.Now().UnixMilli()

				if len(reclaimed) > 0 {
					s.log.Debug("reclaimed messages", "size", len(reclaimed))

					s.handleRange(reclaimed)
				}
			}

			messages, err := s.readFromStream(readBlockMilliseconds)

			if err != nil {
				s.log.Error("failed to read from Redis stream", "error", err)
				s.maybeReconnect()
				return
			}

			if messages != nil {
				s.handleRange(messages)
			}
		}
	}
}

func (s *Streamer) readFromStream(blockTime int64) ([]rueidis.XRangeEntry, error) {
	streamRes := s.client.Do(context.Background(),
		s.client.B().Xreadgroup().Group(s.group, s.consumerName).Block(blockTime).Streams().Key(s.stream).Id(">").Build(),
	)

	res, _ := streamRes.AsXRead()
	err := streamRes.Error()

	if err != nil && !rueidis.IsRedisNil(err) {
		return nil, err
	}

	if res == nil {
		return nil, nil
	}

	if messages, ok := res[s.stream]; ok {
		return messages, nil
	}

	return nil, nil
}

func (s *Streamer) autoclaimMessages(blockTime int64) ([]rueidis.XRangeEntry, error) {
	claimRes := s.client.Do(context.Background(),
		s.client.B().Xautoclaim().Key(s.stream).Group(s.group).Consumer(s.consumerName).MinIdleTime(fmt.Sprintf("%d", blockTime)).Start("0-0").Build(),
	)

	arr, err := claimRes.ToArray()

	if err != nil && !rueidis.IsRedisNil(err) {
		return nil, err
	}

	if arr == nil {
		return nil, nil
	}

	if len(arr) < 2 {
		return nil, fmt.Errorf("autoclaim failed: got %d elements, wanted 2", len(arr))
	}

	ranges, err := arr[1].AsXRange()

	if err != nil {
		return nil, err
	}

	return ranges, nil
}

func (s *Streamer) handleRange(messages []rueidis.XRangeEntry) {
	for _, message := range messages {
		s.log.Debug("received message")

		if err := s.handler(message.FieldValues); err == nil {
			ackRes := s.client.DoMulti(context.Background(),
				s.client.B().Xack().Key(s.stream).Group(s.group).Id(message.ID).Build(),
				s.client.B().Xdel().Key(s.stream).Id(message.ID).Build(),
			)

			ackErr := ackRes[0].Error()

			if ackErr != nil {
				s.log.Error("failed to ack message", "error", ackErr)
			}
		} else {
			s.log.Error("Redis stream handler failed", "error", err)
		}
	}
}

func (s *Streamer) maybeReconnect() {
	if s.reconnectAttempt >= s.config.MaxReconnectAttempts {
		close(s.finishedCh)
		s.log.Error("failed to reconnect to Redis: attempts exceeded")
		return
	}

	s.reconnectAttempt++

	delay := utils.NextRetry(s.reconnectAttempt - 1)

	s.log.Info(fmt.Sprintf("next Redis reconnect attempt in %s", delay))
	time.Sleep(delay)

	s.log.Info("reconnecting to Redis...")

	s.clientMu.Lock()

	if s.client != nil {
		s.client.Close()
		s.client = nil
	}

	s.clientMu.Unlock()

	go s.runReader()
}
