package redisurl_test

import (
	"github.com/soveran/redisurl"
	"github.com/garyburd/redigo/redis"
	"testing"
)

func TestConnect(t *testing.T) {
	c, err := redisurl.Connect()

	if err != nil {
		t.Errorf("Error returned")
	}

	pong, err := redis.String(c.Do("PING"))

	if err != nil {
		t.Errorf("Call to PING returned an error: %v", err)
	}

	if pong != "PONG" {
		t.Errorf("Wanted PONG, got %v\n", pong)
	}
}
