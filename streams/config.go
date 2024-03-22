// This package provides functionality to directly subscribe to streams
// without using channels (a simplified pub/sub mode)
package streams

type Config struct {
	// Secret is a key used to sign and verify streams
	Secret string

	// Public determines if public (unsigned) streams are allowed
	Public bool

	// Whisper determines if whispering is enabled for pub/sub streams
	Whisper bool

	// PubSubChannel is the channel name used for direct pub/sub
	PubSubChannel string

	// Turbo is a flag to enable Turbo Streams support
	Turbo bool

	// TurboSecret is a custom secret key used to verify Turbo Streams
	TurboSecret string

	// CableReady is a flag to enable CableReady support
	CableReady bool

	// CableReadySecret is a custom secret key used to verify CableReady streams
	CableReadySecret string
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
