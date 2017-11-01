package main

import (
	"time"
)

type DisconnectNotifier struct {
	// Limit the number of RPC calls per second
	rate int
	// Call RPC Disconnect for connections
	disconnect chan *Conn
}

func (d *DisconnectNotifier) run() {
	rate := time.Millisecond * time.Duration(1000/d.rate)
	log.Debugf("Disconnect rate %v", rate)
	throttle := time.Tick(rate)

	for {
		select {
		case conn := <-d.disconnect:
			<-throttle
			log.Debugf("Commit disconnect %s %s %v", conn.identifiers, conn.path, conn.headers)
			_, err := rpc.Disconnect(conn.identifiers, SubscriptionsList(conn.subscriptions), conn.path, conn.headers)
			if err != nil {
				log.Errorf("RPC Disconnect Error: %v", err)
			}
		}
	}
}

func (d *DisconnectNotifier) Notify(conn *Conn) {
	d.disconnect <- conn
}

func SubscriptionsList(subs map[string]bool) []string {
	keys := []string{}
	for k := range subs {
		keys = append(keys, k)
	}
	return keys
}
