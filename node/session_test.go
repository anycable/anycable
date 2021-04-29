package node

import (
	"sync"
	"testing"
)

func TestSendRaceConditions(t *testing.T) {
	node := NewMockNode()
	var wg sync.WaitGroup

	for i := 1; i <= 10; i++ {
		session := NewMockSession("123", &node)

		go func() {
			for {
				session.ws.Read() // nolint:errcheck
			}
		}()

		wg.Add(2)
		go func() {
			go func() {
				session.send([]byte("hi!"))
				wg.Done()
			}()

			go func() {
				session.send([]byte("bye"))
				wg.Done()
			}()
		}()

		wg.Add(2)
		go func() {
			go func() {
				session.send([]byte("bye"))
				wg.Done()
			}()

			go func() {
				session.send([]byte("why"))
				wg.Done()
			}()
		}()
	}

	wg.Wait()
}
