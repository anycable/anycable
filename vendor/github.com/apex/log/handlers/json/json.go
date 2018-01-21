// Package json implements a JSON handler.
package json

import (
	j "encoding/json"
	"io"
	"os"
	"sync"

	"github.com/apex/log"
)

// Default handler outputting to stderr.
var Default = New(os.Stderr)

// Handler implementation.
type Handler struct {
	*j.Encoder
	mu sync.Mutex
}

// New handler.
func New(w io.Writer) *Handler {
	return &Handler{
		Encoder: j.NewEncoder(w),
	}
}

// HandleLog implements log.Handler.
func (h *Handler) HandleLog(e *log.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.Encoder.Encode(e)
}
