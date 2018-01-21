// Package es implements an Elasticsearch batch handler. Currently this implementation
// assumes the index format of "logs-YY-MM-DD".
package es

import (
	"io"
	stdlog "log"
	"sync"
	"time"

	"github.com/tj/go-elastic/batch"

	"github.com/apex/log"
)

// TODO(tj): allow dumping logs to stderr on timeout
// TODO(tj): allow custom format that does not include .fields etc
// TODO(tj): allow interval flushes
// TODO(tj): allow explicit Flush() (for Lambda where you have to flush at the end of function)

// Elasticsearch interface.
type Elasticsearch interface {
	Bulk(io.Reader) error
}

// Config for handler.
type Config struct {
	BufferSize int           // BufferSize is the number of logs to buffer before flush (default: 100)
	Format     string        // Format for index
	Client     Elasticsearch // Client for ES
}

// defaults applies defaults to the config.
func (c *Config) defaults() {
	if c.BufferSize == 0 {
		c.BufferSize = 100
	}

	if c.Format == "" {
		c.Format = "logs-06-01-02"
	}
}

// Handler implementation.
type Handler struct {
	*Config

	mu    sync.Mutex
	batch *batch.Batch
}

// New handler with BufferSize
func New(config *Config) *Handler {
	config.defaults()
	return &Handler{
		Config: config,
	}
}

// HandleLog implements log.Handler.
func (h *Handler) HandleLog(e *log.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.batch == nil {
		h.batch = &batch.Batch{
			Index:   time.Now().Format(h.Config.Format),
			Elastic: h.Client,
			Type:    "log",
		}
	}

	h.batch.Add(e)

	if h.batch.Size() >= h.BufferSize {
		go h.flush(h.batch)
		h.batch = nil
	}

	return nil
}

// flush the given `batch` asynchronously.
func (h *Handler) flush(batch *batch.Batch) {
	size := batch.Size()
	start := time.Now()
	stdlog.Printf("log/elastic: flushing %d logs", size)

	if err := batch.Flush(); err != nil {
		stdlog.Printf("log/elastic: failed to flush %d logs: %s", size, err)
	}

	stdlog.Printf("log/elastic: flushed %d logs in %s", size, time.Since(start))
}
