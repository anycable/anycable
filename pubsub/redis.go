package pubsub

import (
	"context"
	"errors"
	"github.com/FZambia/sentinel"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/anycable/anycable-go/node"
	"github.com/apex/log"
	"github.com/gomodule/redigo/redis"
)

const (
	maxReconnectAttempts = 5
)

// RedisSubscriber contains information about Redis pubsub connection
type RedisSubscriber struct {
	node             *node.Node
	url              string
	sentinels        string
	channel          string
	reconnectAttempt int
	log              *log.Entry
}

// NewRedisSubscriber returns new RedisSubscriber struct
func NewRedisSubscriber(node *node.Node, url string, sentinels string, channel string) RedisSubscriber {
	return RedisSubscriber{
		node:             node,
		url:              url,
		sentinels:        sentinels,
		channel:          channel,
		reconnectAttempt: 0,
		log:              log.WithFields(log.Fields{"context": "pubsub"}),
	}
}

// Start connects to Redis and subscribes to the pubsub channel
// if sentinels is set it gets the the master address first
func (s *RedisSubscriber) Start() error {
	// parse URL and check if it is correct
	redisUrl, err := url.Parse(s.url)

	if err != nil {
		return err
	}

	var sntnl *sentinel.Sentinel
	var password string

	if s.sentinels != "" {
		masterName := redisUrl.Hostname()
		password, _ = redisUrl.User.Password()

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

				case <-time.After(30 * time.Second):
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

			if password == "" {
				s.url = redisUrl.Scheme + "://" + masterAddress
			} else {
				s.url = redisUrl.Scheme + "://:" + password + "@" + masterAddress
			}
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
				s.node.HandlePubsub(v.Data)
			case redis.Subscription:
				s.log.Infof("Subscribed to Redis channel: %s\n", v.Channel)
			case error:
				s.log.Errorf("Redis subscription error: %v", v)
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
