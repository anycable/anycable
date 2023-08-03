package broadcast

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/FZambia/sentinel"
	rconfig "github.com/anycable/anycable-go/redis"

	"github.com/apex/log"
	"github.com/gomodule/redigo/redis"
)

// LegacyRedisBroadcaster contains information about Redis pubsub connection
type LegacyRedisBroadcaster struct {
	node                      Handler
	url                       string
	sentinels                 string
	sentinelClient            *sentinel.Sentinel
	sentinelDiscoveryInterval time.Duration
	pingInterval              time.Duration
	channel                   string
	reconnectAttempt          int
	maxReconnectAttempts      int
	uri                       *url.URL
	log                       *log.Entry
	tlsVerify                 bool
}

// NewLegacyRedisBroadcaster returns new RedisSubscriber struct
func NewLegacyRedisBroadcaster(node Handler, config *rconfig.RedisConfig) *LegacyRedisBroadcaster {
	return &LegacyRedisBroadcaster{
		node:                      node,
		url:                       config.URL,
		sentinels:                 config.Sentinels,
		sentinelDiscoveryInterval: time.Duration(config.SentinelDiscoveryInterval),
		channel:                   config.Channel,
		pingInterval:              time.Duration(config.KeepalivePingInterval),
		reconnectAttempt:          0,
		maxReconnectAttempts:      config.MaxReconnectAttempts,
		log:                       log.WithFields(log.Fields{"context": "broadcast", "provider": "redis"}),
		tlsVerify:                 config.TLSVerify,
	}
}

func (LegacyRedisBroadcaster) IsFanout() bool {
	return true
}

// Start connects to Redis and subscribes to the pubsub channel
// if sentinels is set it gets the the master address first
func (s *LegacyRedisBroadcaster) Start(done chan (error)) error {
	// parse URL and check if it is correct
	redisURL, err := url.Parse(s.url)

	s.uri = redisURL

	if err != nil {
		return err
	}

	if s.sentinels != "" {
		masterName := redisURL.Hostname()

		s.log.Debug("Redis sentinel enabled")
		s.log.Debugf("Redis sentinel parameters:  sentinels: %s,  masterName: %s", s.sentinels, masterName)
		sentinels := strings.Split(s.sentinels, ",")
		s.sentinelClient = &sentinel.Sentinel{
			Addrs:      sentinels,
			MasterName: masterName,
			Dial: func(addr string) (redis.Conn, error) {
				timeout := 500 * time.Millisecond

				sentinelHost := addr
				dialOptions := []redis.DialOption{
					redis.DialConnectTimeout(timeout),
					redis.DialReadTimeout(timeout),
					redis.DialReadTimeout(timeout),
					redis.DialTLSSkipVerify(!s.tlsVerify),
				}

				sentinelURI, err := url.Parse(fmt.Sprintf("redis://%s", addr))

				if err == nil {
					sentinelHost = sentinelURI.Host
					password, hasPassword := sentinelURI.User.Password()
					if hasPassword {
						dialOptions = append(dialOptions, redis.DialPassword(password))
					}
				}

				c, err := redis.Dial(
					"tcp",
					sentinelHost,
					dialOptions...,
				)
				if err != nil {
					s.log.Debugf("Failed to connect to sentinel %s", addr)
					return nil, err
				}
				s.log.Debugf("Successfully connected to sentinel %s", addr)
				return c, nil
			},
		}

		go s.discoverSentinels()
	}

	go s.keepalive(done)

	return nil
}

func (s *LegacyRedisBroadcaster) discoverSentinels() {
	defer s.sentinelClient.Close()

	// Periodically discover new Sentinels.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := s.sentinelClient.Discover()
		if err != nil {
			s.log.Warn("Failed to discover sentinels")
		}
		for {
			select {
			case <-ctx.Done():
				return

			case <-time.After(s.sentinelDiscoveryInterval * time.Second):
				err := s.sentinelClient.Discover()
				if err != nil {
					s.log.Warn("Failed to discover sentinels")
				}
			}
		}
	}()
}

func (s *LegacyRedisBroadcaster) keepalive(done chan (error)) {
	for {
		if s.sentinelClient != nil {
			masterAddress, err := s.sentinelClient.MasterAddr()

			if err != nil {
				s.log.Warn("Failed to get master address from sentinel.")
				done <- err
				return
			}
			s.log.Debugf("Got master address from sentinel: %s", masterAddress)

			s.uri.Host = masterAddress
			s.url = s.uri.String()
		}

		if err := s.listen(); err != nil {
			s.log.Warnf("Redis connection failed: %v", err)
		}

		s.reconnectAttempt++

		if s.reconnectAttempt >= s.maxReconnectAttempts {
			done <- errors.New("Redis reconnect attempts exceeded") //nolint:stylecheck
			return
		}

		delay := nextRetry(s.reconnectAttempt)

		s.log.Infof("Next Redis reconnect attempt in %s", delay)
		time.Sleep(delay)

		s.log.Infof("Reconnecting to Redis...")
	}
}

// Shutdown is no-op for Redis
func (s *LegacyRedisBroadcaster) Shutdown(ctx context.Context) error {
	return nil
}

func (s *LegacyRedisBroadcaster) listen() error {
	dialOptions := []redis.DialOption{
		redis.DialTLSSkipVerify(!s.tlsVerify),
	}
	c, err := redis.DialURL(s.url, dialOptions...)

	if err != nil {
		return err
	}

	defer c.Close()

	if s.sentinels != "" {
		if !sentinel.TestRole(c, "master") {
			return errors.New("Failed master role check")
		}
	}

	psc := redis.PubSubConn{Conn: c}
	if err = psc.Subscribe(s.channel); err != nil {
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

	psc.Unsubscribe() //nolint:errcheck
	return <-done
}
