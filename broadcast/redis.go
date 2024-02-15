package broadcast

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	rconfig "github.com/anycable/anycable-go/redis"
	"github.com/anycable/anycable-go/utils"

	nanoid "github.com/matoous/go-nanoid"
	"github.com/redis/rueidis"
)

// RedisBroadcaster represents Redis broadcaster using Redis streams
type RedisBroadcaster struct {
	node   Handler
	config *rconfig.RedisConfig

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

var _ Broadcaster = (*RedisBroadcaster)(nil)

// NewRedisBroadcaster builds a new RedisSubscriber struct
func NewRedisBroadcaster(node Handler, config *rconfig.RedisConfig, l *slog.Logger) *RedisBroadcaster {
	name, _ := nanoid.Nanoid(6)

	return &RedisBroadcaster{
		node:         node,
		config:       config,
		consumerName: name,
		log:          l.With("context", "broadcast").With("provider", "redisx").With("id", name),
		shutdownCh:   make(chan struct{}),
		finishedCh:   make(chan struct{}),
	}
}

func (s *RedisBroadcaster) IsFanout() bool {
	return false
}

func (s *RedisBroadcaster) Start(done chan error) error {
	options, err := s.config.ToRueidisOptions()

	if err != nil {
		return err
	}

	if s.config.IsSentinel() { //nolint:gocritic
		s.log.With("stream", s.config.Channel).With("consumer", s.consumerName).Info(fmt.Sprintf("Starting Redis broadcaster at %v (sentinels)", s.config.Hostnames()))
	} else if s.config.IsCluster() {
		s.log.With("stream", s.config.Channel).With("consumer", s.consumerName).Info(fmt.Sprintf("Starting Redis broadcaster at %v (cluster)", s.config.Hostnames()))
	} else {
		s.log.With("stream", s.config.Channel).With("consumer", s.consumerName).Info(fmt.Sprintf("Starting Redis broadcaster at %s", s.config.Hostname()))
	}

	s.clientOptions = options

	go s.runReader(done)

	return nil
}

func (s *RedisBroadcaster) Shutdown(ctx context.Context) error {
	s.clientMu.RLock()
	defer s.clientMu.RUnlock()

	if s.client == nil {
		return nil
	}

	s.log.Debug("shutting down Redis broadcaster")

	close(s.shutdownCh)

	<-s.finishedCh

	res := s.client.Do(
		context.Background(),
		s.client.B().XgroupDelconsumer().Key(s.config.Channel).Group(s.config.Group).Consumername(s.consumerName).Build(),
	)

	err := res.Error()

	if err != nil {
		s.log.Error("failed to remove Redis stream consumer", "error", err)
	}

	s.client.Close()

	return nil
}

func (s *RedisBroadcaster) initClient() error {
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

func (s *RedisBroadcaster) runReader(done chan (error)) {
	err := s.initClient()

	if err != nil {
		s.log.Error("failed to connect to Redis", "error", err)
		s.maybeReconnect(done)
		return
	}

	// First, create a consumer group for the stream
	err = s.client.Do(context.Background(),
		s.client.B().XgroupCreate().Key(s.config.Channel).Group(s.config.Group).Id("$").Mkstream().Build(),
	).Error()

	if err != nil {
		if redisErr, ok := rueidis.IsRedisErr(err); ok {
			if strings.HasPrefix(redisErr.Error(), "BUSYGROUP") {
				s.log.Debug("Redis consumer group already exists")
			} else {
				s.log.Error("failed to create consumer group", "error", err)
				s.maybeReconnect(done)
				return
			}
		}
	}

	s.reconnectAttempt = 0

	readBlockMilliseconds := s.config.StreamReadBlockMilliseconds
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
					s.maybeReconnect(done)
					return
				}

				lastClaimedAt = time.Now().UnixMilli()

				if len(reclaimed) > 0 {
					s.log.Debug("reclaimed messages", "size", len(reclaimed))

					s.broadcastXrange(reclaimed)
				}
			}

			messages, err := s.readFromStream(readBlockMilliseconds)

			if err != nil {
				s.log.Error("failed to read from Redis stream", "error", err)
				s.maybeReconnect(done)
				return
			}

			if messages != nil {
				s.broadcastXrange(messages)
			}
		}
	}
}

func (s *RedisBroadcaster) readFromStream(blockTime int64) ([]rueidis.XRangeEntry, error) {
	streamRes := s.client.Do(context.Background(),
		s.client.B().Xreadgroup().Group(s.config.Group, s.consumerName).Block(blockTime).Streams().Key(s.config.Channel).Id(">").Build(),
	)

	res, _ := streamRes.AsXRead()
	err := streamRes.Error()

	if err != nil && !rueidis.IsRedisNil(err) {
		return nil, err
	}

	if res == nil {
		return nil, nil
	}

	if messages, ok := res[s.config.Channel]; ok {
		return messages, nil
	}

	return nil, nil
}

func (s *RedisBroadcaster) autoclaimMessages(blockTime int64) ([]rueidis.XRangeEntry, error) {
	claimRes := s.client.Do(context.Background(),
		s.client.B().Xautoclaim().Key(s.config.Channel).Group(s.config.Group).Consumer(s.consumerName).MinIdleTime(fmt.Sprintf("%d", blockTime)).Start("0-0").Build(),
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

func (s *RedisBroadcaster) broadcastXrange(messages []rueidis.XRangeEntry) {
	for _, message := range messages {
		if payload, pok := message.FieldValues["payload"]; pok {
			s.log.Debug("incoming broadcast", "data", payload)

			s.node.HandleBroadcast([]byte(payload))

			ackRes := s.client.DoMulti(context.Background(),
				s.client.B().Xack().Key(s.config.Channel).Group(s.config.Group).Id(message.ID).Build(),
				s.client.B().Xdel().Key(s.config.Channel).Id(message.ID).Build(),
			)

			err := ackRes[0].Error()

			if err != nil {
				s.log.Error("failed to ack message", "error", err)
			}
		}
	}
}

func (s *RedisBroadcaster) maybeReconnect(done chan (error)) {
	if s.reconnectAttempt >= s.config.MaxReconnectAttempts {
		close(s.finishedCh)
		done <- errors.New("failed to reconnect to Redis: attempts exceeded") //nolint:stylecheck
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

	go s.runReader(done)
}
