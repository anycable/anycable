package ds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStream_GenerateCursor(t *testing.T) {
	t.Run("generates non-empty cursor", func(t *testing.T) {
		params := &StreamParams{}
		stream := &Stream{Params: params}

		cursor := stream.NextCursor()
		assert.NotEmpty(t, cursor)
	})

	t.Run("returns greater cursor when client cursor is old", func(t *testing.T) {
		oldCursor := "1"

		params := &StreamParams{Cursor: oldCursor}
		stream := &Stream{Params: params}

		stream.Params.Cursor = oldCursor
		newCursor := stream.NextCursor()

		assert.NotEmpty(t, newCursor)
		assert.Greater(t, newCursor, oldCursor)
	})

	t.Run("ensures monotonic progression for current cursor", func(t *testing.T) {
		params := &StreamParams{}
		stream := &Stream{Params: params}

		currentCursor := stream.NextCursor()

		params.Cursor = currentCursor

		nextCursor := stream.NextCursor()

		assert.Greater(t, nextCursor, currentCursor)
	})

	t.Run("handles invalid cursor gracefully", func(t *testing.T) {
		params := &StreamParams{Cursor: "not-a-number"}
		stream := &Stream{Params: params}

		cursor := stream.NextCursor()
		assert.NotEmpty(t, cursor)
	})
}
