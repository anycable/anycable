// This package provides functionality to directly subscribe to streams
// without using channels (a simplified pub/sub mode)
package streams

type Config struct {
	// Secret is a key used to sign and verify streams
	Secret string

	// Public determines if public (unsigned) streams are allowed
	Public bool

	// PubSubChannel is the channel name used for direct pub/sub
	PubSubChannel string

	// Turbo is a flag to enable Turbo Streams support
	Turbo bool

	// CableReady is a flag to enable CableReady support
	CableReady bool
}

// NewConfig returns a new Config with the given key
func NewConfig() Config {
	return Config{
		PubSubChannel: "$pubsub",
	}
}
