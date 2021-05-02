package pubsub

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/anycable/anycable-go/node"
	"github.com/apex/log"
	"github.com/go-redis/redis/v8"
)

const (
	maxReconnectAttempts = 5
)

// RedisConfig contains Redis pubsub adapter configuration
type RedisConfig struct {
	// Redis instance URL or master name in case of sentinels usage
	URL string
	// Redis channel to subscribe to
	Channel string
	// List of Redis Sentinel addresses
	Sentinels string
	// Redis Sentinel discovery interval (seconds). Deprecated
	SentinelDiscoveryInterval int
	// Redis keepalive ping interval (seconds). Deprecated
	KeepalivePingInterval int
}

// NewRedisConfig builds a new config for Redis pubsub
func NewRedisConfig() RedisConfig {
	return RedisConfig{}
}

// RedisSubscriber contains information about Redis pubsub connection
type RedisSubscriber struct {
	node      node.AppNode
	url       string
	sentinels string
	channel   string
	log       *log.Entry
	pubsub    *redis.PubSub
}

// NewRedisSubscriber returns new RedisSubscriber struct
func NewRedisSubscriber(node node.AppNode, config *RedisConfig) *RedisSubscriber {
	return &RedisSubscriber{
		node:      node,
		url:       config.URL,
		sentinels: config.Sentinels,
		channel:   config.Channel,
		log:       log.WithFields(log.Fields{"context": "pubsub"}),
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

	clientOptions := &redis.UniversalOptions{
		Addrs:      []string{redisURL.Host},
		MaxRetries: maxReconnectAttempts,
	}

	if redisURL.Scheme == "rediss" {
		h, _, err := net.SplitHostPort(redisURL.Host)
		if err != nil {
			return err
		}
		clientOptions.TLSConfig = &tls.Config{ServerName: h, InsecureSkipVerify: true} // #nosec
	}

	if s.sentinels != "" {
		masterName := redisURL.Hostname()
		s.log.Debug("Redis sentinel enabled")
		s.log.Debugf("Redis sentinel parameters:  sentinels: %s,  masterName: %s", s.sentinels, masterName)

		var addrs []string
		for _, addr := range strings.Split(s.sentinels, ",") {
			if sentinelURI, err := url.Parse(fmt.Sprintf("redis://%s", addr)); err == nil {
				addr = sentinelURI.Host
				password, hasPassword := sentinelURI.User.Password()
				if hasPassword {
					clientOptions.SentinelPassword = password
				}
			}
			addrs = append(addrs, addr)
		}
		clientOptions.Addrs = addrs
		clientOptions.MasterName = masterName
	}

	rdb := redis.NewUniversalClient(clientOptions)
	s.pubsub = rdb.Subscribe(context.Background(), s.channel)

	ch := s.pubsub.Channel()
	go func() {
		for msg := range ch {
			s.log.Debugf("Incoming pubsub message from Redis: %s", msg.Payload)
			s.node.HandlePubSub([]byte(msg.Payload))
		}
	}()

	return nil
}

// Shutdown is no-op for Redis
func (s *RedisSubscriber) Shutdown() (err error) {
	if s.pubsub != nil {
		err = s.pubsub.Close()
	}

	return
}
