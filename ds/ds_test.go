package ds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateCursor(t *testing.T) {
	t.Run("generates non-empty cursor", func(t *testing.T) {
		cursor := GenerateCursor("")
		assert.NotEmpty(t, cursor)
	})

	t.Run("returns greater cursor when client cursor is old", func(t *testing.T) {
		// Use a very old cursor value
		oldCursor := "1"
		newCursor := GenerateCursor(oldCursor)

		assert.NotEmpty(t, newCursor)
		// New cursor should be numerically greater
		assert.Greater(t, newCursor, oldCursor)
	})

	t.Run("ensures monotonic progression for current cursor", func(t *testing.T) {
		// First get current cursor
		currentCursor := GenerateCursor("")

		// When client echoes the same cursor, server should return a greater one
		nextCursor := GenerateCursor(currentCursor)

		assert.Greater(t, nextCursor, currentCursor)
	})

	t.Run("handles invalid cursor gracefully", func(t *testing.T) {
		// Invalid cursor should be ignored and a valid cursor returned
		cursor := GenerateCursor("not-a-number")
		assert.NotEmpty(t, cursor)
	})
}
