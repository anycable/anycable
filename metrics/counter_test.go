package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounter(t *testing.T) {
	cnt := NewCounter("test", "")
	assert.Equal(t, int64(0), cnt.IntervalValue())
	for i := 0; i < 1000; i++ {
		cnt.Inc()
	}
	assert.Equal(t, int64(1000), cnt.Value())
	cnt.Add(500)
	assert.Equal(t, int64(1500), cnt.Value())
	cnt.UpdateDelta()
	cnt.Inc()
	assert.Equal(t, int64(1501), cnt.Value())
	assert.Equal(t, int64(1500), cnt.IntervalValue())
	cnt.UpdateDelta()
	assert.Equal(t, int64(1501), cnt.Value())
	assert.Equal(t, int64(1), cnt.IntervalValue())
	cnt.UpdateDelta()
	assert.Equal(t, int64(1501), cnt.Value())
	assert.Equal(t, int64(0), cnt.IntervalValue())
}
