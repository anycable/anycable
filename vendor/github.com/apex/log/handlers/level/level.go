// Package level implements a level filter handler.
package level

import "github.com/apex/log"

// Handler implementation.
type Handler struct {
	Level   log.Level
	Handler log.Handler
}

// New handler.
func New(h log.Handler, level log.Level) *Handler {
	return &Handler{
		Level:   level,
		Handler: h,
	}
}

// HandleLog implements log.Handler.
func (h *Handler) HandleLog(e *log.Entry) error {
	if e.Level < h.Level {
		return nil
	}

	return h.Handler.HandleLog(e)
}
