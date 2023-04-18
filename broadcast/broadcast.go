// This package contains different message broadcast handler implemenentations.
// Broadcast handler is responsible for consumeing broadcast messages from the outer world and
// routing them to the application node.
//
// NOTE: There could be multiple broadcast handlers running at the same time.
package broadcast

type Broadcaster interface {
	Start(done chan (error)) error
	Shutdown() error
	// Returns true if the broadcaster fan-outs the same event
	// to all nodes. Such subscriber shouldn't be used with real pub/sub
	// engines (which are responsible for message distribution)
	IsFanout() bool
}

type Handler interface {
	// Handle broadcast message delivered only to this node (and pass it through the broker)
	// (Used by single-node broadcasters)
	HandleBroadcast(json []byte)
	// Handle broadcast message delivered to all nodes
	// (Used by fan-out broadcasters)
	HandlePubSub(json []byte)
}
