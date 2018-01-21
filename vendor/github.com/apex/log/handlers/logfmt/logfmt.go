// Package logfmt implements a "logfmt" format handler.
package logfmt

import (
	"io"
	"os"
	"sync"

	"github.com/apex/log"
	"github.com/go-logfmt/logfmt"
)

// Default handler outputting to stderr.
var Default = New(os.Stderr)

// Handler implementation.
type Handler struct {
	mu  sync.Mutex
	enc *logfmt.Encoder
}

// New handler.
func New(w io.Writer) *Handler {
	return &Handler{
		enc: logfmt.NewEncoder(w),
	}
}

// HandleLog implements log.Handler.
func (h *Handler) HandleLog(e *log.Entry) error {
	names := e.Fields.Names()

	h.mu.Lock()
	defer h.mu.Unlock()

	h.enc.EncodeKeyval("timestamp", e.Timestamp)
	h.enc.EncodeKeyval("level", e.Level.String())
	h.enc.EncodeKeyval("message", e.Message)

	for _, name := range names {
		h.enc.EncodeKeyval(name, e.Fields.Get(name))
	}

	h.enc.EndRecord()

	return nil
}
