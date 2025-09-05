package node

import (
	"context"

	"github.com/anycable/anycable-go/netpoll"
	"github.com/anycable/anycable-go/utils"
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

		s.metrics.GaugeIncrement(metricsPollPoolPendingNum)
		rerr := s.pollPool.ScheduleContext(ctx, func() {
			s.metrics.GaugeDecrement(metricsPollPoolPendingNum)
			s.metrics.GaugeIncrement(metricsPollPoolRunningNum)
			defer func() {
				s.metrics.GaugeDecrement(metricsPollPoolRunningNum)
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
				s.EnqueueReadMessage(message)
			}
		})

		if rerr != nil {
			s.metrics.GaugeDecrement(metricsPollPoolPendingNum)
			s.metrics.CounterIncrement(metricsPollPoolTimeoutsTotal)
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

// EnqueueReadMessage adds a new incoming message to the queue and activates the read routine (from the pool)
func (s *Session) EnqueueReadMessage(message []byte) {
	if s.readQueue.Closed() {
		return
	}

	pendingTotal := s.readQueue.Size()
	// TODO: Add limit and configuration
	maxPending := uint64(0)
	// s.mu.Lock()
	// maxPending := s.maxPendingSize
	// s.mu.Unlock()

	if maxPending > 0 && pendingTotal >= maxPending {
		s.readQueue.Clear()
		// s.mu.Lock()
		// s.maxPendingSize = 0
		// s.mu.Unlock()

		s.Log.Debug("chatty client detected, disconnecting", "pending", pendingTotal, "max", maxPending)

		// s.metrics.CounterIncrement(metricsDisconnectedSlowClients)
		// s.DisconnectWithMessage(common.NewDisconnectMessage(common.SLOW_CLIENT_REASON, true), common.SLOW_CLIENT_REASON)
		return
	}

	size := len(message)

	item := utils.Item[[]byte]{Data: message, Size: uint64(size)}

	if !s.readQueue.Add(item) {
		defer s.Disconnect("scheduling read failed", ws.CloseAbnormalClosure)
	}

	s.ensureReadWorkerRunning()
}

// ReadMessages takes messages from the read queue and process them
func (s *Session) ReadMessages() {
	for {
		pendingLen := s.readQueue.Len()
		if pendingLen <= 0 {
			return
		}

		item, ok := s.readQueue.Remove()
		if !ok {
			return
		}

		err := s.ReadMessage(item.Data)

		if err != nil {
			return
		}
	}
}

func (s *Session) ensureReadWorkerRunning() {
	s.rmu.Lock()

	if s.readPool == nil {
		s.rmu.Unlock()
		return
	}

	if s.readWorkerRunning || s.readWorkerScheduled {
		s.rmu.Unlock()
		return
	}

	s.readWorkerScheduled = true

	s.rmu.Unlock()

	s.metrics.GaugeIncrement(metricsReadPoolPendingNum)
	s.readPool.ScheduleContext(s.closeCtx, func() { // nolint:errcheck
		s.metrics.GaugeDecrement(metricsReadPoolPendingNum)
		s.metrics.GaugeIncrement(metricsReadPoolRunningNum)
		defer func() {
			s.metrics.GaugeDecrement(metricsReadPoolRunningNum)
		}()

		s.rmu.Lock()
		if s.readWorkerRunning {
			s.rmu.Unlock()
			return
		}

		s.readWorkerScheduled = false
		s.readWorkerRunning = true
		s.rmu.Unlock()

		s.ReadMessages()

		s.rmu.Lock()
		s.readWorkerRunning = false
		s.rmu.Unlock()
	})
}
