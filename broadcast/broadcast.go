// This package contains different message broadcast handler implemenentations.
// Broadcast handler is responsible for consumeing broadcast messages from the outer world and
// routing them to the application node.
//
// NOTE: There could be multiple broadcast handlers running at the same time.
package broadcast

type Broadcaster interface {
	Start(done chan (error)) error
	Shutdown() error
}

type Handler interface {
	HandlePubSub(json []byte)
}
