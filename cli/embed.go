package cli

import (
	"context"
	"fmt"
	"net/http"

	"github.com/anycable/anycable-go/broadcast"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/version"
)

// A minimal interface to the underlying Runner for embedding AnyCable into your own Go HTTP application.
type Embedded struct {
	n *node.Node
	r *Runner
}

// WebSocketHandler returns an HTTP handler to serve WebSocket connections via AnyCable.
func (e *Embedded) WebSocketHandler() (http.Handler, error) {
	wsHandler, err := e.r.websocketHandlerFactory(e.n, e.r.config, e.r.log)

	if err != nil {
		return nil, err
	}

	return wsHandler, nil
}

// SSEHandler returns an HTTP handler to serve SSE connections via AnyCable.
// Please, provide your HTTP server's shutdown context to terminate SSE connections gracefully
// on server shutdown.
func (e *Embedded) SSEHandler(ctx context.Context) (http.Handler, error) {
	sseHandler, err := e.r.defaultSSEHandler(e.n, ctx, e.r.config)

	if err != nil {
		return nil, err
	}

	return sseHandler, nil
}

// HTTPBroadcastHandler returns an HTTP handler to process broadcasting requests
func (e *Embedded) HTTPBroadcastHandler() (http.Handler, error) {
	broadcaster := broadcast.NewHTTPBroadcaster(e.n, &e.r.config.HTTPBroadcast, e.r.log)

	err := broadcaster.Prepare()

	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(broadcaster.Handler), nil
}

// Shutdown stops the AnyCable node gracefully.
func (e *Embedded) Shutdown(ctx context.Context) error {
	for _, shutdownable := range e.r.shutdownables {
		if err := shutdownable.Shutdown(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Embed starts the application without setting up HTTP handlers, signalts, etc.
// You can use it to embed AnyCable into your own Go HTTP application.
func (r *Runner) Embed() (*Embedded, error) {
	r.announceDebugMode()
	mrubySupport := r.initMRuby()

	r.log.Info(fmt.Sprintf("Starting embedded %s %s%s (open file limit: %s)", r.name, version.Version(), mrubySupport, utils.OpenFileLimit()))

	appNode, err := r.runNode()
	if err != nil {
		return nil, err
	}

	embed := &Embedded{n: appNode, r: r}

	go r.startMetrics(r.metrics)

	return embed, nil
}
