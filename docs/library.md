# Using anycable-go as a library

You can use AnyCable-Go as a library to build custom real-time applications.

> Read ["AnyCable off Rails: connecting Twilio streams with Hanami"](https://evilmartians.com/chronicles/anycable-goes-off-rails-connecting-twilio-streams-with-hanami) to learn how we've integrated Twilio Streams with a Hanami application via AnyCable-Go.

Why building a WebSocket application with AnyCable-Go (and not other Go libraries)?

- Connect your application to Ruby/Rails apps with ease by using AnyCable RPC protocol.
- Many features out-of-the-box including different pub/sub adapters (including [embedded NATS](./embedded_nats.md)), built-in instrumentation.
- Bulletproof code, which has been used production for years.

To get started with an application development with AnyCable-Go, you can use our template repository: [anycable-go-scaffold](https://github.com/anycable/anycable-go-scaffold).

## Embedding

You can also embed AnyCable into your existing web application in case you want to serve AnyCable WebSocket/SSE connections via the same HTTP server as other requests (e.g., if you build a smart reverse-proxy).

Here is a minimal example Go code (you can find the full and up-to-date version [here](https://github.com/anycable/anycable-go/blob/master/cmd/embedded-cable/main.go)):

```go
package main

import (
	"net/http"

	"github.com/anycable/anycable-go/cli"
)

func main() {
	opts := []cli.Option{
		cli.WithName("AnyCable"),
		cli.WithDefaultRPCController(),
		cli.WithDefaultBroker(),
		cli.WithDefaultSubscriber(),
		cli.WithDefaultBroadcaster(),
	}

	c := cli.NewConfig()
	runner, _ := cli.NewRunner(c, opts)
	anycable, _ := runner.Embed()

	wsHandler, _ := anycable.WebSocketHandler()
	http.Handle("/cable", wsHandler)

	http.ListenAndServe(":8080", nil)
}
```
