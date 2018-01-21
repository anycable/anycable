// Package implements a Graylog-backed handler.
package graylog

import (
	"github.com/apex/log"
	"github.com/aphistic/golf"
)

// Handler implementation.
type Handler struct {
	logger *golf.Logger
	client *golf.Client
}

// New handler.
// Connection string should be in format "udp://<ip_address>:<port>".
// Server should have GELF input enabled on that port.
func New(url string) (*Handler, error) {
	c, err := golf.NewClient()
	if err != nil {
		return nil, err
	}

	err = c.Dial(url)
	if err != nil {
		return nil, err
	}

	l, err := c.NewLogger()
	if err != nil {
		return nil, err
	}

	return &Handler{
		logger: l,
		client: c,
	}, nil
}

// HandleLog implements log.Handler.
func (h *Handler) HandleLog(e *log.Entry) error {
	switch e.Level {
	case log.DebugLevel:
		return h.logger.Dbgm(e.Fields, e.Message)
	case log.InfoLevel:
		return h.logger.Infom(e.Fields, e.Message)
	case log.WarnLevel:
		return h.logger.Warnm(e.Fields, e.Message)
	case log.ErrorLevel:
		return h.logger.Errm(e.Fields, e.Message)
	case log.FatalLevel:
		return h.logger.Critm(e.Fields, e.Message)
	}

	return nil
}

// Closes connection to server, flushing message queue.
func (h *Handler) Close() error {
	return h.client.Close()
}
