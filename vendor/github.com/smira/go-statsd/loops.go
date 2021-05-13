package statsd

/*

Copyright (c) 2017 Andrey Smirnov

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

*/

import (
	"context"
	"net"
	"sync/atomic"
	"time"
)

// flushLoop makes sure metrics are flushed every flushInterval
func (t *transport) flushLoop(flushInterval time.Duration) {
	var flushC <-chan time.Time

	if flushInterval > 0 {
		flushTicker := time.NewTicker(flushInterval)
		defer flushTicker.Stop()
		flushC = flushTicker.C
	}

	for {
		select {
		case <-t.shutdown:
			t.bufLock.Lock()
			if len(t.buf) > 0 {
				t.flushBuf(len(t.buf))
			}
			t.bufLock.Unlock()

			close(t.sendQueue)
			return
		case <-flushC:
			t.bufLock.Lock()
			if len(t.buf) > 0 {
				t.flushBuf(len(t.buf))
			}
			t.bufLock.Unlock()
		}
	}
}

// sendLoop handles packet delivery over UDP and periodic reconnects
func (t *transport) sendLoop(addr string, reconnectInterval, retryTimeout time.Duration, log SomeLogger) {
	var (
		sock       net.Conn
		err        error
		reconnectC <-chan time.Time
	)

	defer t.shutdownWg.Done()

	if reconnectInterval > 0 {
		reconnectTicker := time.NewTicker(reconnectInterval)
		defer reconnectTicker.Stop()
		reconnectC = reconnectTicker.C
	}

RECONNECT:
	// Attempt to connect
	sock, err = func() (net.Conn, error) {
		// Dial with context which is aborted when client is shut down
		ctx, ctxCancel := context.WithCancel(context.Background())
		defer ctxCancel()

		go func() {
			select {
			case <-t.shutdown:
				ctxCancel()
			case <-ctx.Done():
			}
		}()

		var d net.Dialer
		return d.DialContext(ctx, "udp", addr)
	}()

	if err != nil {
		log.Printf("[STATSD] Error connecting to server: %s", err)
		goto WAIT
	}

	for {
		select {
		case buf, ok := <-t.sendQueue:
			// Get a buffer from the queue
			if !ok {
				_ = sock.Close() // nolint: gosec
				return
			}

			if len(buf) > 0 {
				// cut off \n in the end
				_, err := sock.Write(buf[0 : len(buf)-1])
				if err != nil {
					log.Printf("[STATSD] Error writing to socket: %s", err)
					_ = sock.Close() // nolint: gosec
					goto WAIT
				}
			}

			// return buffer to the pool
			select {
			case t.bufPool <- buf:
			default:
				// pool is full, let GC handle the buf
			}
		case <-reconnectC:
			_ = sock.Close() // nolint: gosec
			goto RECONNECT
		}
	}

WAIT:
	// Wait for a while
	select {
	case <-time.After(retryTimeout):
		goto RECONNECT
	case <-t.shutdown:
	}

	// drain send queue waiting for flush loops to terminate
	for range t.sendQueue {
	}
}

// reportLoop reports periodically number of packets lost
func (t *transport) reportLoop(reportInterval time.Duration, log SomeLogger) {
	defer t.shutdownWg.Done()

	reportTicker := time.NewTicker(reportInterval)
	defer reportTicker.Stop()

	for {
		select {
		case <-t.shutdown:
			return
		case <-reportTicker.C:
			lostPeriod := atomic.SwapInt64(&t.lostPacketsPeriod, 0)
			if lostPeriod > 0 {
				log.Printf("[STATSD] %d packets lost (overflow)", lostPeriod)
			}
		}
	}
}
