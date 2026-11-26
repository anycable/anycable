package ds

import (
	"math"
	"strconv"
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

		anotherStream := &Stream{Params: &StreamParams{Cursor: currentCursor}}

		nextCursor := anotherStream.NextCursor()

		assert.Greater(t, nextCursor, currentCursor)
	})

	t.Run("handles invalid cursor gracefully", func(t *testing.T) {
		params := &StreamParams{Cursor: "not-a-number"}
		stream := &Stream{Params: params}

		cursor := stream.NextCursor()
		assert.NotEmpty(t, cursor)
	})

	t.Run("ignores cursors that would overflow", func(t *testing.T) {
		for _, val := range []int64{math.MaxInt64, math.MaxInt64 - 60} {
			stream := &Stream{Params: &StreamParams{Cursor: strconv.FormatInt(val, 10)}}

			cursor, err := strconv.ParseInt(stream.NextCursor(), 10, 64)

			assert.NoError(t, err)
			assert.Positive(t, cursor)
		}
	})

	t.Run("advances large valid cursors", func(t *testing.T) {
		val := int64(math.MaxInt64 - 61)
		stream := &Stream{Params: &StreamParams{Cursor: strconv.FormatInt(val, 10)}}

		cursor, err := strconv.ParseInt(stream.NextCursor(), 10, 64)

		assert.NoError(t, err)
		assert.Greater(t, cursor, val)
	})
}
