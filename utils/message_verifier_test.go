package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageVerifier(t *testing.T) {
	verifier := NewMessageVerifier("s3Krit")

	// Turbo.signed_stream_verifier_key = 's3Krit'
	// Turbo::StreamsChannel.signed_stream_name([:chat, "2021"])
	example := "ImNoYXQ6MjAyMSI=--f9ee45dbccb1da04d8ceb99cc820207804370ba0d06b46fc3b8b373af1315628"

	res, err := verifier.Verified(example)

	assert.NoError(t, err)
	assert.Equal(t, "chat:2021", res)
}
