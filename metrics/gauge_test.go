package metrics

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGauge(t *testing.T) {
	g := NewGauge("test", "")
	assert.Equal(t, uint64(0), g.Value())
	g.Set(20)
	assert.Equal(t, uint64(20), g.Value())
}

func TestGaugeIncDec(t *testing.T) {
	g := NewGauge("test", "")

	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)

		go func() {
			g.Inc()
			wg.Done()
		}()
	}

	for i := 0; i < 13; i++ {
		wg.Add(1)

		go func() {
			g.Dec()
			wg.Done()
		}()
	}

	wg.Wait()

	assert.Equal(t, uint64(7), g.Value())
}
