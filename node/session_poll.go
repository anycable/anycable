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

	s.Log.Debug("adding descriptor to poller")

	retried := false

ADD:
	err = poller.Start(desc, func(ev netpoll.Event) {
		if ev&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
			poller.Stop(desc) // nolint:errcheck
			desc.Close()

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
				poller.Stop(desc) // nolint:errcheck
				s.disconnectNow("Read closed", ws.CloseNormalClosure)
				return
			}

			if rerr != nil {
				s.Log.Debug("WebSocket read failed", "error", rerr)
				poller.Stop(desc) // nolint:errcheck
				desc.Close()
				s.disconnectNow("Read error", ws.CloseAbnormalClosure)
				callback()
				return
			}

			defer poller.Resume(desc) // nolint:errcheck

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
			poller.Resume(desc) // nolint:errcheck
		}
	})

	// There could be a race condition when we haven't removed the closed
	// socket from the poller; in this case, we force-remove it and try adding
	// the descriptor again
	if err == netpoll.ErrRegistered && !retried {
		retried = true
		s.Log.Debug("retried to add fd")
		poller.Stop(desc) // nolint:errcheck
		goto ADD
	}

	return err
}
