package admin

import (
	"errors"
	"fmt"
	"strings"

	"github.com/anycable/anycable-go/common"
)

const (
	logsStream = "logs"
)

// Real-time data streams management (logs, metrics)

func (app *App) HandleSubscribe(sid string, stream string) error {
	if stream == logsStream {
		return app.handleLogs(sid)
	}

	return errors.New("unknown stream")
}

func (app *App) handleLogs(sid string) error {
	if app.tracer == nil {
		return errors.New("logs stream is not available")
	}

	app.tracer.Subscribe()

	app.log.Debug("subscribed to logs stream", "sid", sid)

	return nil
}

func (app *App) HandleDisconnect(sid string, subscriptions []string) error {
	for _, stream := range subscriptions {
		switch stream { // nolint:gocritic
		case logsStream:
			app.handleLogsUnsubscribe(sid)
		}
	}

	return nil
}

func (app *App) handleLogsUnsubscribe(sid string) {
	if app.tracer == nil {
		return
	}

	app.tracer.Unsubscribe()

	app.log.Debug("unsubscribed from logs stream", "sid", sid)
}

func (app *App) broadcastLogs(msg string) {
	// Turn the logs into a JSON array to make EventSource happy
	msg = strings.ReplaceAll(msg, "}\n{", "},{")
	data := fmt.Sprintf("[%s]", msg)
	app.node.Broadcast(&common.StreamMessage{Stream: logsStream, Data: data})
}
