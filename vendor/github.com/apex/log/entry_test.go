package log

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEntry_WithFields(t *testing.T) {
	a := NewEntry(nil)
	assert.Nil(t, a.Fields)

	b := a.WithFields(Fields{"foo": "bar"})
	assert.Equal(t, Fields{}, a.mergedFields())
	assert.Equal(t, Fields{"foo": "bar"}, b.mergedFields())

	c := a.WithFields(Fields{"foo": "hello", "bar": "world"})

	e := c.finalize(InfoLevel, "upload")
	assert.Equal(t, e.Message, "upload")
	assert.Equal(t, e.Fields, Fields{"foo": "hello", "bar": "world"})
	assert.Equal(t, e.Level, InfoLevel)
	assert.NotEmpty(t, e.Timestamp)
}

func TestEntry_WithField(t *testing.T) {
	a := NewEntry(nil)
	b := a.WithField("foo", "bar")
	assert.Equal(t, Fields{}, a.mergedFields())
	assert.Equal(t, Fields{"foo": "bar"}, b.mergedFields())
}

func TestEntry_WithError(t *testing.T) {
	a := NewEntry(nil)
	b := a.WithError(fmt.Errorf("boom"))
	assert.Equal(t, Fields{}, a.mergedFields())
	assert.Equal(t, Fields{"error": "boom"}, b.mergedFields())
}

func TestEntry_WithErrorFields(t *testing.T) {
	a := NewEntry(nil)
	b := a.WithError(errFields("boom"))
	assert.Equal(t, Fields{}, a.mergedFields())
	assert.Equal(t, Fields{
		"error":  "boom",
		"reason": "timeout",
	}, b.mergedFields())
}

type errFields string

func (ef errFields) Error() string {
	return string(ef)
}

func (ef errFields) Fields() Fields {
	return Fields{"reason": "timeout"}
}
