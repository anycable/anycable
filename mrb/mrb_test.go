//go:build (darwin && mrb) || (linux && mrb)

package mrb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadString(t *testing.T) {
	engine := NewEngine()

	engine.LoadString(
		`
		module Example
			def self.add(a, b)
				a + b
			end
		end
		`,
	)

	result, err := engine.Eval("Example.add(20, 22)")

	assert.Nil(t, err)
	assert.Equal(t, 42, result.Fixnum())
}
