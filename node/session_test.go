package node

import (
	"testing"
)

func TestSendRaceConditions(t *testing.T) {
	done := make(chan bool)

	for i := 1; i <= 10; i++ {
		session := NewMockSession("123")
		// small buffer channel
		session.send = make(chan []byte, 1)

		go func() {
			go func() {
				session.Send([]byte("hi!"))
				done <- true
			}()

			go func() {
				session.Send([]byte("bye"))
				done <- true
			}()
		}()

		go func() {
			go func() {
				session.Send([]byte("bye"))
				done <- true
			}()

			go func() {
				session.Send([]byte("why"))
				done <- true
			}()
		}()
	}

	for i := 1; i <= 40; i++ {
		<-done
	}
}
