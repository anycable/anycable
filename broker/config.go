package broker

import (
	"fmt"
	"strings"
)

type Config struct {
	// Adapter name
	Adapter string `toml:"adapter"`
	// For how long to keep history in seconds
	HistoryTTL int64 `toml:"history_ttl"`
	// Max size of messages to keep in the history per stream
	HistoryLimit int `toml:"history_limit"`
	// Sessions cache TTL in seconds (after disconnect)
	SessionsTTL int64 `toml:"sessions_ttl"`
	// Presence expire TTL in seconds (after disconnect)
	PresenceTTL int64 `toml:"presence_ttl"`
}

func NewConfig() Config {
	return Config{
		// 5 minutes by default
		HistoryTTL: 5 * 60,
		// 100 msgs by default
		HistoryLimit: 100,
		// 5 minutes by default
		SessionsTTL: 5 * 60,
		// 15 seconds by default
		PresenceTTL: 15,
	}
}

func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# Broker backend adapter\n")
	if c.Adapter == "" {
		result.WriteString("# adapter = \"memory\"\n")
	} else {
		result.WriteString(fmt.Sprintf("adapter = \"%s\"\n", c.Adapter))
	}

	result.WriteString("# For how long to keep streams history (seconds)\n")
	result.WriteString(fmt.Sprintf("history_ttl = %d\n", c.HistoryTTL))

	result.WriteString("# Max number of messages to keep in a stream history\n")
	result.WriteString(fmt.Sprintf("history_limit = %d\n", c.HistoryLimit))

	result.WriteString("# For how long to store sessions state for resumeability (seconds)\n")
	result.WriteString(fmt.Sprintf("sessions_ttl = %d\n", c.SessionsTTL))

	result.WriteString("# For how long to keep presence information after session disconnect (seconds)\n")
	result.WriteString(fmt.Sprintf("presence_ttl = %d\n", c.PresenceTTL))

	result.WriteString("\n")

	return result.String()
}
