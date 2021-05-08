package encoders

import (
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
)

func TestJSONEncoder(t *testing.T) {
	coder := JSON{}

	t.Run(".Encode", func(t *testing.T) {
		msg := &common.Reply{Type: "test", Identifier: "test_channel", Message: "hello"}

		expected := []byte("{\"type\":\"test\",\"identifier\":\"test_channel\",\"message\":\"hello\"}")

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
	})

	t.Run(".EncodeTransmission", func(t *testing.T) {
		msg := "{\"type\":\"test\",\"identifier\":\"test_channel\",\"message\":\"hello\"}"
		expected := []byte(msg)

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
	})

	t.Run(".Decode", func(t *testing.T) {
		msg := []byte("{\"command\":\"test\",\"identifier\":\"test_channel\",\"data\":\"hello\"}")

		actual, err := coder.Decode(msg)

		assert.NoError(t, err)
		assert.Equal(t, actual.Command, "test")
		assert.Equal(t, actual.Identifier, "test_channel")
		assert.Equal(t, actual.Data, "hello")
	})
}
