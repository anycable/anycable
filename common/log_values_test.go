package common

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilteredIdentifier(t *testing.T) {
	digest := "2fe0d3c5d4ebd07ff8f1bc2ba6d93d1e2b1ad8a0ab2dbd7a5da5ebd4a19a4b91"

	for _, tc := range []struct {
		name     string
		input    string
		expected string
	}{
		{
			"signed stream",
			`{"channel":"$pubsub","signed_stream_name":"ImNoYXQ6MjAyMSI=--` + digest + `"}`,
			`{"channel":"$pubsub","signed_stream_name":"ImNoYXQ6MjAyMSI=--2f***91"}`,
		},
		{
			"turbo stream with spaces",
			`{"channel": "Turbo::StreamsChannel", "signed_stream_name": "ImNoYXQ6MjAyMSI=--` + digest + `"}`,
			`{"channel": "Turbo::StreamsChannel", "signed_stream_name": "ImNoYXQ6MjAyMSI=--2f***91"}`,
		},
		{
			"cable ready stream",
			`{"channel":"CableReady::Stream","identifier":"ImNoYXQ6MjAyMSI=--` + digest + `"}`,
			`{"channel":"CableReady::Stream","identifier":"ImNoYXQ6MjAyMSI=--2f***91"}`,
		},
		{
			"public stream",
			`{"channel":"$pubsub","stream_name":"chat:2021"}`,
			`{"channel":"$pubsub","stream_name":"chat:2021"}`,
		},
		{
			"regular channel",
			`{"channel":"ChatChannel","id":"2021"}`,
			`{"channel":"ChatChannel","id":"2021"}`,
		},
		{
			"signed stream without digest",
			`{"channel":"$pubsub","signed_stream_name":"ImNoYXQ6MjAyMSI="}`,
			`{"channel":"$pubsub","signed_stream_name":"ImNoYXQ6MjAyMSI="}`,
		},
		{
			"confirmation message",
			ConfirmationMessage(`{"channel":"$pubsub","signed_stream_name":"ImNoYXQ6MjAyMSI=--` + digest + `"}`),
			`{"type":"confirm_subscription","identifier":"{\"channel\":\"$pubsub\",\"signed_stream_name\":\"ImNoYXQ6MjAyMSI=--2f***91\"}"}`,
		},
		{
			"multiple signed streams",
			`{"a":"x--` + digest + `","b":"y--` + digest + `"}`,
			`{"a":"x--2f***91","b":"y--2f***91"}`,
		},
		{
			"invalid JSON",
			`{"channel":"$pubsub","signed_stream_name":"ImNoYXQ6MjAyMSI=--` + digest + `",`,
			`{"channel":"$pubsub","signed_stream_name":"ImNoYXQ6MjAyMSI=--2f***91",`,
		},
		{
			"not a JSON",
			"chat_1",
			"chat_1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FilteredIdentifier(tc.input).LogValue().String())
		})
	}
}

func TestCommandResultLogValue_masksSignedStreams(t *testing.T) {
	digest := "2fe0d3c5d4ebd07ff8f1bc2ba6d93d1e2b1ad8a0ab2dbd7a5da5ebd4a19a4b91"
	identifier := `{"channel":"$pubsub","signed_stream_name":"ImNoYXQ6MjAyMSI=--` + digest + `"}`

	res := &CommandResult{
		Status:        SUCCESS,
		Transmissions: []string{ConfirmationMessage(identifier)},
		IState:        map[string]string{"user_email": "john@example.com"},
	}

	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("result", "res", res)

	output := buf.String()

	assert.NotContains(t, output, digest[:8])
	assert.NotContains(t, output, "john@example.com")
	assert.Contains(t, output, "res.istate=[user_email]")
}

func TestFilteredTransmissions(t *testing.T) {
	digest := "2fe0d3c5d4ebd07ff8f1bc2ba6d93d1e2b1ad8a0ab2dbd7a5da5ebd4a19a4b91"
	long := `{"signed_stream_name":"eDE=--` + digest + `","padding":"` + strings.Repeat("x", 200) + `"}`

	res := filteredTransmissions([]string{long})

	// Masking happens before truncation
	assert.True(t, strings.HasPrefix(res[0].LogValue().String(), `{"signed_stream_name":"eDE=--2f***91","padding":"xxx`))
	assert.Contains(t, res[0].LogValue().String(), "...(")
}
