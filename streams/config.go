// This package provides functionality to directly subscribe to streams
// without using channels (a simplified pub/sub mode)
package streams

import (
	"fmt"
	"strings"
)

type Config struct {
	// Secret is a key used to sign and verify streams
	Secret string `toml:"secret"`

	// Public determines if public (unsigned) streams are allowed
	Public bool `toml:"public"`

	// Whisper determines if whispering is enabled for pub/sub streams
	Whisper bool `toml:"whisper"`

	// PubSubChannel is the channel name used for direct pub/sub
	PubSubChannel string `toml:"pubsub_channel"`

	// Turbo is a flag to enable Turbo Streams support
	Turbo bool `toml:"turbo"`

	// TurboSecret is a custom secret key used to verify Turbo Streams
	TurboSecret string `toml:"turbo_secret"`

	// CableReady is a flag to enable CableReady support
	CableReady bool `toml:"cable_ready"`

	// CableReadySecret is a custom secret key used to verify CableReady streams
	CableReadySecret string `toml:"cable_ready_secret"`
}

// NewConfig returns a new Config with the given key
func NewConfig() Config {
	return Config{
		PubSubChannel: "$pubsub",
	}
}

func (c Config) GetTurboSecret() string {
	if c.TurboSecret != "" {
		return c.TurboSecret
	}

	return c.Secret
}

func (c Config) GetCableReadySecret() string {
	if c.CableReadySecret != "" {
		return c.CableReadySecret
	}

	return c.Secret
}

func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# Secret key used to sign and verify pub/sub streams\n")
	if c.Secret != "" {
		result.WriteString(fmt.Sprintf("secret = \"%s\"\n", c.Secret))
	} else {
		result.WriteString("# secret = \"\"\n")
	}

	result.WriteString("# Enable public (unsigned) streams\n")
	if c.Public {
		result.WriteString("public = true\n")
	} else {
		result.WriteString("# public = true\n")
	}

	result.WriteString("# Enable whispering support for pub/sub streams\n")
	if c.Whisper {
		result.WriteString("whisper = true\n")
	} else {
		result.WriteString("# whisper = true\n")
	}

	result.WriteString("# Name of the channel used for pub/sub\n")
	result.WriteString(fmt.Sprintf("pubsub_channel = \"%s\"\n", c.PubSubChannel))

	result.WriteString("# Enable Turbo Streams support\n")
	if c.Turbo {
		result.WriteString("turbo = true\n")
	} else {
		result.WriteString("# turbo = true\n")
	}

	result.WriteString("# Custom secret key used to verify Turbo Streams\n")
	if c.TurboSecret != "" {
		result.WriteString(fmt.Sprintf("turbo_secret = \"%s\"\n", c.TurboSecret))
	} else {
		result.WriteString("# turbo_secret = \"\"\n")
	}

	result.WriteString("# Enable CableReady support\n")
	if c.CableReady {
		result.WriteString("cable_ready = true\n")
	} else {
		result.WriteString("# cable_ready = true\n")
	}

	result.WriteString("# Custom secret key used to verify CableReady streams\n")
	if c.CableReadySecret != "" {
		result.WriteString(fmt.Sprintf("cable_ready_secret = \"%s\"\n", c.CableReadySecret))
	} else {
		result.WriteString("# cable_ready_secret = \"\"\n")
	}

	result.WriteString("\n")

	return result.String()
}
