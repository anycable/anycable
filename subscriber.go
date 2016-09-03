package main

import (
  "encoding/json"

  "github.com/soveran/redisurl"
  "github.com/garyburd/redigo/redis"
)

type Subscriber struct {
  host string
  channel string
}

func (s *Subscriber) run() {
  c, err := redisurl.ConnectToURL(s.host)

  if err != nil {
    panic(err)
  }

  psc := redis.PubSubConn{c}
  psc.Subscribe(s.channel)
  for {
      switch v := psc.Receive().(type) {
      case redis.Message:
          log.Debugf("[Redis] channel %s: message: %s\n", v.Channel, v.Data)
          msg := &StreamMessage{}
          if err := json.Unmarshal(v.Data, &msg); err != nil {
            log.Debugf("Unknown message: %s", v.Data)
          }else{
            log.Debugf("Broadcast %v", msg)
            hub.stream_broadcast <- msg
          }
      case redis.Subscription:
          log.Debugf("%s: %s %d\n", v.Channel, v.Kind, v.Count)
      case error:
          break
      }
  }
}
