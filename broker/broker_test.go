package broker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamsTracker_AddRemove(t *testing.T) {
	tracker := NewStreamsTracker()

	t.Run("Add first subscription", func(t *testing.T) {
		isNew := tracker.Add("stream1")
		assert.True(t, isNew, "First subscription should return true")
		assert.True(t, tracker.Has("stream1"))
	})

	t.Run("Add second subscription to same stream", func(t *testing.T) {
		isNew := tracker.Add("stream1")
		assert.False(t, isNew, "Second subscription should return false")
		assert.True(t, tracker.Has("stream1"))
	})

	t.Run("Add third subscription to same stream", func(t *testing.T) {
		isNew := tracker.Add("stream1")
		assert.False(t, isNew, "Third subscription should return false")
		assert.True(t, tracker.Has("stream1"))
	})

	t.Run("Remove first subscription (count: 3 -> 2)", func(t *testing.T) {
		isLast := tracker.Remove("stream1")
		assert.False(t, isLast, "Should not be last when count > 1")
		assert.True(t, tracker.Has("stream1"), "Stream should still exist")
	})

	t.Run("Remove second subscription (count: 2 -> 1)", func(t *testing.T) {
		isLast := tracker.Remove("stream1")
		assert.False(t, isLast, "Should not be last when count = 1 (will be deleted on next remove)")
		assert.True(t, tracker.Has("stream1"), "Stream should still exist")
	})

	t.Run("Remove third subscription (count: 1 -> 0)", func(t *testing.T) {
		isLast := tracker.Remove("stream1")
		assert.True(t, isLast, "Should be last when count = 1")
		assert.False(t, tracker.Has("stream1"), "Stream should be deleted")
	})

	t.Run("Remove non-existent stream", func(t *testing.T) {
		isLast := tracker.Remove("nonexistent")
		assert.False(t, isLast, "Should return false for non-existent stream")
	})
}

func TestStreamsTracker_MultipleStreams(t *testing.T) {
	tracker := NewStreamsTracker()

	assert.True(t, tracker.Add("stream1"))
	assert.True(t, tracker.Add("stream2"))
	assert.False(t, tracker.Add("stream1"))
	assert.False(t, tracker.Add("stream2"))

	assert.True(t, tracker.Has("stream1"))
	assert.True(t, tracker.Has("stream2"))

	assert.False(t, tracker.Remove("stream1"))
	assert.True(t, tracker.Has("stream1"), "stream1 should still exist after first remove")

	assert.True(t, tracker.Remove("stream1"))
	assert.False(t, tracker.Has("stream1"), "stream1 should be deleted")

	assert.True(t, tracker.Has("stream2"))

	assert.False(t, tracker.Remove("stream2"))
	assert.True(t, tracker.Remove("stream2"))
	assert.False(t, tracker.Has("stream2"))
}

func TestStreamsTracker_ReferenceCount(t *testing.T) {
	tracker := NewStreamsTracker()

	for i := 0; i < 5; i++ {
		isNew := tracker.Add("popular_stream")
		if i == 0 {
			assert.True(t, isNew, "First subscription should be new")
		} else {
			assert.False(t, isNew, "Subsequent subscriptions should not be new")
		}
	}

	for i := 0; i < 4; i++ {
		isLast := tracker.Remove("popular_stream")
		assert.False(t, isLast, "Should not be last until count reaches 1")
		assert.True(t, tracker.Has("popular_stream"), "Stream should still exist")
	}

	isLast := tracker.Remove("popular_stream")
	assert.True(t, isLast, "Should be last when removing final subscription")
	assert.False(t, tracker.Has("popular_stream"), "Stream should be deleted")
}
