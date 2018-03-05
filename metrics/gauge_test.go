package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGauge(t *testing.T) {
	g := NewGauge()
	assert.Equal(t, int64(0), g.Value())
	g.Set(20)
	assert.Equal(t, int64(20), g.Value())
}
