package node

import (
	"context"

	"github.com/anycable/anycable-go/netpoll"
	"github.com/anycable/anycable-go/ws"

	gobwas "github.com/gobwas/ws"
)

// ServeWithPoll register the connection within a netpoll and subscribes to Read/Close events
func (s *Session) ServeWithPoll(poller netpoll.Poller, callback func()) error {
	wsConn := s.conn.(*ws.Connection)

	fd := wsConn.Fd()

	// Connection has been already closed
	if fd < 0 {
		s.Log.Debug("no fd, has been already closed")
		return nil
	}

	desc, err := netpoll.HandleReadOnce(fd)

	if err != nil {
		s.Log.Debug("couldn't obtain a descriptor for polling")
		return err
	}

	// Ensure the desc is removed from the registry (for reusability)
	wsConn.OnClose(func() {
		poller.Stop(desc) // nolint:errcheck
	})

	onEvent := func(ev netpoll.Event) {
		if ev&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
			wsConn.IfOpen(func() {
				poller.Stop(desc) // nolint:errcheck
			})

			if ev&(netpoll.EventErr) != 0 {
				s.Log.Debug("descriptor closed with error")
				s.metrics.CounterIncrement(metricsAbnormalSocketClosure)
				s.cleanup()
			} else {
				s.Log.Debug("descriptor closed", "event", ev)
				s.disconnectNow("Closed", ws.CloseNormalClosure)
			}

			callback()
			return
		}

		wsconn := s.conn.Descriptor()

		ctx, cancel := context.WithTimeout(context.Background(), s.readWaitTimeout)
		defer cancel()

		s.metrics.GaugeIncrement(metricsReadPoolPendingNum)
		rerr := s.readPool.ScheduleContext(ctx, func() {
			s.metrics.GaugeDecrement(metricsReadPoolPendingNum)
			s.metrics.GaugeIncrement(metricsReadPoolRunningNum)
			defer func() {
				s.metrics.GaugeDecrement(metricsReadPoolRunningNum)
			}()

			message, op, rerr := ws.ReadClientData(wsconn)

			if op == gobwas.OpClose {
				s.Log.Debug("WebSocket closed", "error", rerr)
				wsConn.IfOpen(func() {
					poller.Stop(desc) // nolint:errcheck
				})
				s.disconnectNow("Read closed", ws.CloseNormalClosure)
				return
			}

			if rerr != nil {
				s.Log.Debug("WebSocket read failed", "error", rerr)
				wsConn.IfOpen(func() {
					poller.Stop(desc) // nolint:errcheck
				})
				s.disconnectNow("Read error", ws.CloseAbnormalClosure)
				callback()
				return
			}

			// The connection can be closed while we handle the message
			defer wsConn.IfOpen(func() {
				poller.Resume(desc) // nolint:errcheck
			})

			if op == gobwas.OpPong {
				s.HandleNativePong()
			}

			if message != nil {
				s.ReadMessage(message) // nolint:errcheck
			}
		})

		if rerr != nil {
			s.metrics.GaugeDecrement(metricsReadPoolPendingNum)
			s.metrics.CounterIncrement(metricsReadPoolTimeoutsTotal)
			// Discard frame
			_, op, _ := ws.DiscardClientData(wsconn)
			s.Log.Error("timed out to process WebSocket frame, discarded", "op", op)
			wsConn.IfOpen(func() {
				poller.Resume(desc) // nolint:errcheck
			})
		}
	}

	s.Log.Debug("adding descriptor to poller")

	opened := wsConn.IfOpen(func() {
		err = poller.Start(desc, onEvent)

		// A closed connection must have been removed from the poller (see OnClose above),
		// so this is not expected; still, we force-remove the descriptor and try again
		if err == netpoll.ErrRegistered {
			s.Log.Debug("fd is already registered, retrying")
			poller.Stop(desc) // nolint:errcheck
			err = poller.Start(desc, onEvent)
		}
	})

	if !opened {
		s.Log.Debug("closed before polling started")
		return nil
	}

	return err
}
