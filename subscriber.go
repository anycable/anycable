package main

import (
	"encoding/json"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/soveran/redisurl"
)

type Subscriber struct {
	host    string
	channel string
}

func (s *Subscriber) run() {
	for {
		if err := s.listen(); err != nil {
			log.Errorf("Redis failure: %v", err)
		}

		time.Sleep(5 * time.Second)
		log.Infof("Reconnecting to Redis...")
	}
}

func (s *Subscriber) listen() error {
	c, err := redisurl.ConnectToURL(s.host)

	if err != nil {
		return err
	}

	defer c.Close()

	psc := redis.PubSubConn{Conn: c}
	if err := psc.Subscribe(s.channel); err != nil {
		log.Criticalf("failed to subscribe to Redis channel: %v", err)
		return err
	}

	done := make(chan error, 1)

	go func() {
		for {
			switch v := psc.Receive().(type) {
			case redis.Message:
				log.Debugf("[Redis] channel %s: message: %s\n", v.Channel, v.Data)
				msg := &StreamMessage{}
				if err := json.Unmarshal(v.Data, &msg); err != nil {
					log.Debugf("Unknown message: %s", v.Data)
				} else {
					log.Debugf("Broadcast %v", msg)
					hub.stream_broadcast <- msg
				}
			case redis.Subscription:
				log.Infof("Subscribed to Redis channel: %s\n", v.Channel)
			case error:
				log.Criticalf("Redis subscription error: %v", v)
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
