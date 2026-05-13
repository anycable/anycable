package lib_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/anycable/anycable-go/etc/benchi/lib"
	"github.com/stretchr/testify/assert"
)

func TestAccumulator_RaceClean(t *testing.T) {
	acc := lib.NewAccumulator()

	const goroutines = 100
	const bumpsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			id := strconv.Itoa(g % 10) // 10 distinct client IDs
			for range bumpsPerGoroutine {
				acc.Bump(id)
			}
		}(g)
	}
	wg.Wait()

	snap := acc.Snapshot()
	total := 0
	for _, v := range snap {
		total += v
	}
	assert.Equal(t, goroutines*bumpsPerGoroutine, total)
}

func TestAccumulator_SnapshotIsCopy(t *testing.T) {
	acc := lib.NewAccumulator()
	acc.Bump("a")
	acc.Bump("a")
	acc.Bump("b")

	snap1 := acc.Snapshot()
	snap1["a"] = 999
	snap1["c"] = 42

	snap2 := acc.Snapshot()
	assert.Equal(t, 2, snap2["a"], "Snapshot must not share storage with future snapshots")
	assert.Equal(t, 1, snap2["b"])
	_, hasC := snap2["c"]
	assert.False(t, hasC, "mutating one snapshot must not leak into the next")
}
