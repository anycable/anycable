//go:build linux

package netpoll

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestEpoll_Tokens(t *testing.T) {
	setup := func(t *testing.T) (*Epoll, int, int, chan EpollEvent) {
		ep, err := EpollCreate(nil)
		require.NoError(t, err)
		t.Cleanup(func() { ep.Close() }) // nolint:errcheck

		fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
		require.NoError(t, err)
		t.Cleanup(func() {
			unix.Close(fds[0])
			unix.Close(fds[1])
		})

		events := make(chan EpollEvent, 10)

		require.NoError(t, ep.Add(fds[0], EPOLLIN|EPOLLONESHOT, func(ev EpollEvent) {
			if ev&_EPOLLCLOSED == 0 {
				events <- ev
			}
		}))

		return ep, fds[0], fds[1], events
	}

	t.Run("Delivers events after re-arming with Mod", func(t *testing.T) {
		ep, fd, peer, events := setup(t)

		_, err := unix.Write(peer, []byte("a"))
		require.NoError(t, err)

		select {
		case <-events:
		case <-time.After(time.Second):
			t.Fatal("no event")
		}

		require.NoError(t, ep.Mod(fd, EPOLLIN|EPOLLONESHOT))

		select {
		case <-events:
		case <-time.After(time.Second):
			t.Fatal("no event after Mod")
		}
	})

	t.Run("Ignores events with a token of another registration", func(t *testing.T) {
		ep, fd, peer, events := setup(t)

		// Re-register the fd in the kernel with a foreign token:
		// this is what a stale event of a previous registration looks like
		require.NoError(t, unix.EpollCtl(ep.fd, unix.EPOLL_CTL_MOD, fd, &unix.EpollEvent{
			Events: uint32(EPOLLIN | EPOLLONESHOT),
			Fd:     int32(fd),
			Pad:    1 << 20,
		}))

		_, err := unix.Write(peer, []byte("a"))
		require.NoError(t, err)

		select {
		case <-events:
			t.Fatal("callback called for a stale event")
		case <-time.After(100 * time.Millisecond):
		}

		// Mod restores the token; the data is still there
		require.NoError(t, ep.Mod(fd, EPOLLIN|EPOLLONESHOT))

		select {
		case <-events:
		case <-time.After(time.Second):
			t.Fatal("no event after Mod")
		}
	})
}
