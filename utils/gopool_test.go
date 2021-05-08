package utils

import (
	"bytes"
	"runtime"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkerRespawn(t *testing.T) {
	pool := NewGoPool(1)

	var wg sync.WaitGroup

	resChan := make(chan uint64, 2)

	wg.Add(1)

	pool.Schedule(func() {
		resChan <- getGID()
	})

	for i := 0; i < workerRespawnThreshold; i++ {
		pool.Schedule(func() {})
	}

	pool.Schedule(func() {
		resChan <- getGID()
		wg.Done()
	})

	initial := <-resChan
	current := <-resChan

	assert.NotEqual(t, initial, current)
}

// Get current goroutine ID
// Source: https://blog.sgmansfield.com/2015/12/goroutine-ids/
func getGID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
