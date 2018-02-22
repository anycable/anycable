package pubsub

import (
	"errors"
	"math/rand"
	"net/url"
	"time"

	"github.com/anycable/anycable-go/node"
	"github.com/apex/log"
	"github.com/garyburd/redigo/redis"
	"github.com/soveran/redisurl"
)

const (
	maxReconnectAttempts = 5
)

// RedisSubscriber contains information about Redis pubsub connection
type RedisSubscriber struct {
	node             *node.Node
	url              string
	channel          string
	reconnectAttempt int
}

// NewRedisSubscriber returns new RedisSubscriber struct
func NewRedisSubscriber(node *node.Node, url string, channel string) RedisSubscriber {
	return RedisSubscriber{node: node, url: url, channel: channel, reconnectAttempt: 0}
}

// Start connects to Redis and subscribes to the pubsub channel
func (s *RedisSubscriber) Start() error {
	// Check that URL is correct first
	_, err := url.Parse(s.url)

	if err != nil {
		return err
	}

	for {
		if err := s.listen(); err != nil {
			log.Warnf("Redis connection failed: %v", err)
		}

		s.reconnectAttempt++

		if s.reconnectAttempt >= maxReconnectAttempts {
			return errors.New("Redis reconnect attempts exceeded")
		}

		delay := nextRetry(s.reconnectAttempt)

		log.Infof("Next Redis reconnect attempt in %s", delay)
		time.Sleep(delay)

		log.Infof("Reconnecting to Redis...")
	}
}

func (s *RedisSubscriber) listen() error {
	c, err := redisurl.ConnectToURL(s.url)

	if err != nil {
		return err
	}

	defer c.Close()

	psc := redis.PubSubConn{Conn: c}
	if err := psc.Subscribe(s.channel); err != nil {
		log.Errorf("Failed to subscribe to Redis channel: %v", err)
		return err
	}

	s.reconnectAttempt = 0

	done := make(chan error, 1)

	go func() {
		for {
			switch v := psc.Receive().(type) {
			case redis.Message:
				log.Debugf("Incoming pubsub message from Redis: %s", v.Data)
				s.node.HandlePubsub(v.Data)
			case redis.Subscription:
				log.Infof("Subscribed to Redis channel: %s\n", v.Channel)
			case error:
				log.Errorf("Redis subscription error: %v", v)
				done <- v
				break
			}
		}
	}()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

loop:
	for err == nil {
		select {
		case <-ticker.C:
			if err = psc.Ping(""); err != nil {
				break loop
			}
		case err := <-done:
			// Return error from the receive goroutine.
			return err
		}
	}

	psc.Unsubscribe()
	return <-done
}

func nextRetry(step int) time.Duration {
	secs := (step * step) + (rand.Intn(step*4) * (step + 1))
	return time.Duration(secs) * time.Second
}
