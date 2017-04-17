package main

import (
	"encoding/json"
	"time"
)

type Pinger struct {
	interval time.Duration
	ticker   *time.Ticker
	cmd      chan string
	count    uint32
}

type PingReply struct {
	Type    string      `json:"type"`
	Message interface{} `json:"message"`
}

func (p *PingReply) toJSON() []byte {
	jsonStr, err := json.Marshal(&p)
	if err != nil {
		panic("Failed to build JSON")
	}
	return jsonStr
}

func NewPinger(interval time.Duration) *Pinger {
	return &Pinger{count: 0, interval: interval, cmd: make(chan string)}
}

func (p *Pinger) run() {
	log.Debugf("Ping interval %v", p.interval)
	p.ticker = time.NewTicker(p.interval)
	defer p.ticker.Stop()

	for {
		select {
		case <-p.ticker.C:
			if p.count > 0 {
				log.Debugf("Ping will be sent to %v", p.count)
				app.BroadcastAll((&PingReply{Type: "ping", Message: time.Now().Unix()}).toJSON())
				log.Debugf("Ping was sent to %v", p.count)
			}
		case cmd := <-p.cmd:
			if cmd == "incr" {
				p.count += 1
			} else {
				p.count -= 1
			}
			log.Debugf("Ping count %v", p.count)
		}
	}
}

func (p *Pinger) Increment() {
	log.Debugf("Increment ping")
	p.cmd <- "incr"
}

func (p *Pinger) Decrement() {
	log.Debugf("Decrement ping")
	p.cmd <- "decr"
}
