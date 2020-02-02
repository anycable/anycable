package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionStateMergeConnectionState(t *testing.T) {
	env := NewSessionEnv("", nil)
	(*env.ConnectionState)["a"] = "old"

	t.Run("when adding and rewriting", func(t *testing.T) {
		env.MergeConnectionState(&map[string]string{"a": "new", "b": "super new"})

		assert.Len(t, *env.ConnectionState, 2)
		assert.Equal(t, "new", (*env.ConnectionState)["a"])
		assert.Equal(t, "super new", (*env.ConnectionState)["b"])
	})
}
