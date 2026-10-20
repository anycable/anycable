//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

package netpoll

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestPoller_WatchHangup(t *testing.T) {
	setup := func(t *testing.T) (Poller, *Desc, int, chan Event) {
		poller, err := New(nil)
		require.NoError(t, err)

		fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
		require.NoError(t, err)
		t.Cleanup(func() {
			unix.Close(fds[0])
			unix.Close(fds[1])
		})

		desc, err := HandleReadOnce(fds[0])
		require.NoError(t, err)

		events := make(chan Event, 10)
		require.NoError(t, poller.Start(desc, func(ev Event) { events <- ev }))

		// Receive the first data event, so the one-shot registration is disarmed
		// (as when a read worker handles a message)
		_, err = unix.Write(fds[1], []byte("a"))
		require.NoError(t, err)

		select {
		case ev := <-events:
			require.NotZero(t, ev&EventRead)
		case <-time.After(time.Second):
			t.Fatal("no event")
		}

		return poller, desc, fds[1], events
	}

	t.Run("Reports a hang-up but not data", func(t *testing.T) {
		poller, desc, peer, events := setup(t)

		require.NoError(t, poller.WatchHangup(desc))

		_, err := unix.Write(peer, []byte("b"))
		require.NoError(t, err)

		select {
		case ev := <-events:
			t.Fatalf("unexpected event for data: %v", ev)
		case <-time.After(100 * time.Millisecond):
		}

		unix.Close(peer)

		select {
		case ev := <-events:
			assert.NotZero(t, ev&(EventReadHup|EventHup), "event: %v", ev)
		case <-time.After(time.Second):
			t.Fatal("no event for hang-up")
		}
	})

	t.Run("Resume reports data received while watching for hang-ups", func(t *testing.T) {
		poller, desc, peer, events := setup(t)

		require.NoError(t, poller.WatchHangup(desc))

		_, err := unix.Write(peer, []byte("b"))
		require.NoError(t, err)

		select {
		case ev := <-events:
			t.Fatalf("unexpected event for data: %v", ev)
		case <-time.After(100 * time.Millisecond):
		}

		require.NoError(t, poller.Resume(desc))

		select {
		case ev := <-events:
			assert.NotZero(t, ev&EventRead)
			assert.Zero(t, ev&(EventReadHup|EventHup), "event: %v", ev)
		case <-time.After(time.Second):
			t.Fatal("no event after Resume")
		}
	})
}
