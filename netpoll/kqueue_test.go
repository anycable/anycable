//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package netpoll

import (
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestKqueue_Tokens(t *testing.T) {
	readOnce := Kevents{{Filter: EVFILT_READ, Flags: EV_ADD | EV_ONESHOT}}

	setup := func(t *testing.T) (*Kqueue, int, int, chan Kevent) {
		kq, err := KqueueCreate(nil)
		require.NoError(t, err)

		fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
		require.NoError(t, err)
		t.Cleanup(func() {
			unix.Close(fds[0])
			unix.Close(fds[1])
		})

		events := make(chan Kevent, 10)

		require.NoError(t, kq.Add(fds[0], readOnce, 1, func(ev Kevent) { events <- ev }))

		return kq, fds[0], fds[1], events
	}

	t.Run("Delivers events after re-arming with Mod", func(t *testing.T) {
		kq, fd, peer, events := setup(t)

		_, err := unix.Write(peer, []byte("a"))
		require.NoError(t, err)

		select {
		case <-events:
		case <-time.After(time.Second):
			t.Fatal("no event")
		}

		require.NoError(t, kq.Mod(fd, readOnce, 1))

		select {
		case <-events:
		case <-time.After(time.Second):
			t.Fatal("no event after Mod")
		}
	})

	t.Run("Ignores events with a token of another registration", func(t *testing.T) {
		kq, fd, peer, events := setup(t)

		// Re-register the fd in the kernel with a foreign token:
		// this is what a stale event of a previous registration looks like
		stale := evGet(fd, Kevent{Filter: EVFILT_READ, Flags: EV_ADD | EV_ONESHOT}, kqueueTokenBase|uintptr(1<<20))
		_, err := unix.Kevent(kq.fd, []unix.Kevent_t{stale}, nil, nil)
		require.NoError(t, err)
		require.Equal(t, kqueueTokenBase|uintptr(1<<20), *(*uintptr)(unsafe.Pointer(&stale.Udata)))

		_, err = unix.Write(peer, []byte("a"))
		require.NoError(t, err)

		select {
		case <-events:
			t.Fatal("callback called for a stale event")
		case <-time.After(100 * time.Millisecond):
		}

		// Mod restores the token; the data is still there
		require.NoError(t, kq.Mod(fd, readOnce, 1))

		select {
		case ev := <-events:
			assert.Equal(t, KeventFilter(EVFILT_READ), ev.Filter)
		case <-time.After(time.Second):
			t.Fatal("no event after Mod")
		}
	})
}
