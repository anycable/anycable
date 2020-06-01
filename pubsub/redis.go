package pubsub

import (
	"context"
	"errors"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/FZambia/sentinel"

	"github.com/anycable/anycable-go/node"
	"github.com/apex/log"
	"github.com/gomodule/redigo/redis"
)

const (
	maxReconnectAttempts     = 5
	defaultKeepaliveInterval = 30
)

// RedisConfig contains Redis pubsub adapter configuration
type RedisConfig struct {
	// Redis instance URL or master name in case of sentinels usage
	URL string
	// Redis channel to subscribe to
	Channel string
	// List of Redis Sentinel addresses
	Sentinels string
	// Redis Sentinel discovery interval (seconds)
	SentinelDiscoveryInterval int
	// Redis keepalive ping interval (seconds)
	KeepalivePingInterval int
}

// NewRedisConfig builds a new config for Redis pubsub
func NewRedisConfig() RedisConfig {
	return RedisConfig{KeepalivePingInterval: defaultKeepaliveInterval}
}

// RedisSubscriber contains information about Redis pubsub connection
type RedisSubscriber struct {
	node                      node.AppNode
	url                       string
	sentinels                 string
	sentinelDiscoveryInterval time.Duration
	pingInterval              time.Duration
	channel                   string
	reconnectAttempt          int
	log                       *log.Entry
}

// NewRedisSubscriber returns new RedisSubscriber struct
func NewRedisSubscriber(node node.AppNode, config *RedisConfig) *RedisSubscriber {
	return &RedisSubscriber{
		node:                      node,
		url:                       config.URL,
		sentinels:                 config.Sentinels,
		sentinelDiscoveryInterval: time.Duration(config.SentinelDiscoveryInterval),
		channel:                   config.Channel,
		pingInterval:              time.Duration(config.KeepalivePingInterval),
		reconnectAttempt:          0,
		log:                       log.WithFields(log.Fields{"context": "pubsub"}),
	}
}

// Start connects to Redis and subscribes to the pubsub channel
// if sentinels is set it gets the the master address first
func (s *RedisSubscriber) Start() error {
	// parse URL and check if it is correct
	redisURL, err := url.Parse(s.url)

	if err != nil {
		return err
	}

	var sntnl *sentinel.Sentinel

	if s.sentinels != "" {
		masterName := redisURL.Hostname()

		s.log.Debug("Redis sentinel enabled")
		s.log.Debugf("Redis sentinel parameters:  sentinels: %s,  masterName: %s", s.sentinels, masterName)
		sentinels := strings.Split(s.sentinels, ",")
		sntnl = &sentinel.Sentinel{
			Addrs:      sentinels,
			MasterName: masterName,
			Dial: func(addr string) (redis.Conn, error) {
				timeout := 500 * time.Millisecond

				c, err := redis.Dial(
					"tcp",
					addr,
					redis.DialConnectTimeout(timeout),
					redis.DialReadTimeout(timeout),
					redis.DialReadTimeout(timeout),
				)
				if err != nil {
					s.log.Debugf("Failed to connect to sentinel %s", addr)
					return nil, err
				}
				s.log.Debugf("Successfully connected to sentinel %s", addr)
				return c, nil
			},
		}

		defer sntnl.Close()

		// Periodically discover new Sentinels.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			err := sntnl.Discover()
			if err != nil {
				s.log.Warn("Failed to discover sentinels")
			}
			for {
				select {
				case <-ctx.Done():
					return

				case <-time.After(s.sentinelDiscoveryInterval * time.Second):
					err := sntnl.Discover()
					if err != nil {
						s.log.Warn("Failed to discover sentinels")
					}
				}
			}
		}()
	}

	for {

		if s.sentinels != "" {
			masterAddress, err := sntnl.MasterAddr()

			if err != nil {
				s.log.Warn("Failed to get master address from sentinel.")
				return err
			}
			s.log.Debugf("Got master address from sentinel: %s", masterAddress)

			redisURL.Host = masterAddress
			s.url = redisURL.String()
		}

		if err := s.listen(); err != nil {
			s.log.Warnf("Redis connection failed: %v", err)
		}

		s.reconnectAttempt++

		if s.reconnectAttempt >= maxReconnectAttempts {
			return errors.New("Redis reconnect attempts exceeded")
		}

		delay := nextRetry(s.reconnectAttempt)

		s.log.Infof("Next Redis reconnect attempt in %s", delay)
		time.Sleep(delay)

		s.log.Infof("Reconnecting to Redis...")
	}
}

// Shutdown is no-op for Redis
func (s *RedisSubscriber) Shutdown() {
}

func (s *RedisSubscriber) listen() error {

	c, err := redis.DialURL(s.url)

	if err != nil {
		return err
	}

	if s.sentinels != "" {
		if !sentinel.TestRole(c, "master") {
			return errors.New("Failed master role check")
		}
	}

	defer c.Close()

	psc := redis.PubSubConn{Conn: c}
	if err := psc.Subscribe(s.channel); err != nil {
		s.log.Errorf("Failed to subscribe to Redis channel: %v", err)
		return err
	}

	s.reconnectAttempt = 0

	done := make(chan error, 1)

	go func() {
		for {
			switch v := psc.Receive().(type) {
			case redis.Message:
				s.log.Debugf("Incoming pubsub message from Redis: %s", v.Data)
				s.node.HandlePubSub(v.Data)
			case redis.Subscription:
				s.log.Infof("Subscribed to Redis channel: %s\n", v.Channel)
			case error:
				s.log.Errorf("Redis subscription error: %v", v)
				done <- v
				break
			}
		}
	}()

	ticker := time.NewTicker(s.pingInterval * time.Second)
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
