package broker

type Config struct {
	// For how long to keep history in seconds
	HistoryTTL int64
	// Max size of messages to keep in the history per stream
	HistoryLimit int
	// Sessions cache TTL in seconds (after disconnect)
	SessionsTTL int64
}

func NewConfig() Config {
	return Config{
		// 5 minutes by default
		HistoryTTL: 5 * 60,
		// 100 msgs by default
		HistoryLimit: 100,
		// 5 minutes by default
		SessionsTTL: 5 * 60,
	}
}
