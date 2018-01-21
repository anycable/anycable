// Package delta provides a log handler which times the delta
// between each log call, useful for debug output for command-line
// programs.
package delta

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/apex/log"
	"github.com/aybabtme/rgbterm"
	"github.com/tj/go-spin"
)

// TODO: move colors and share in text handler etc

// color function.
type colorFunc func(string) string

// gray string.
func gray(s string) string {
	return rgbterm.FgString(s, 150, 150, 150)
}

// blue string.
func blue(s string) string {
	return rgbterm.FgString(s, 77, 173, 247)
}

// cyan string.
func cyan(s string) string {
	return rgbterm.FgString(s, 34, 184, 207)
}

// green string.
func green(s string) string {
	return rgbterm.FgString(s, 0, 200, 255)
}

// red string.
func red(s string) string {
	return rgbterm.FgString(s, 194, 37, 92)
}

// yellow string.
func yellow(s string) string {
	return rgbterm.FgString(s, 252, 196, 25)
}

// Colors mapping.
var Colors = [...]colorFunc{
	log.DebugLevel: gray,
	log.InfoLevel:  blue,
	log.WarnLevel:  yellow,
	log.ErrorLevel: red,
	log.FatalLevel: red,
}

// Strings mapping.
var Strings = [...]string{
	log.DebugLevel: "DEBU",
	log.InfoLevel:  "INFO",
	log.WarnLevel:  "WARN",
	log.ErrorLevel: "ERRO",
	log.FatalLevel: "FATA",
}

// Default handler.
var Default = New(os.Stderr)

// Handler implementation.
type Handler struct {
	entries chan *log.Entry
	start   time.Time
	spin    *spin.Spinner
	prev    *log.Entry
	done    chan struct{}
	w       io.Writer
}

// New handler.
func New(w io.Writer) *Handler {
	h := &Handler{
		entries: make(chan *log.Entry),
		done:    make(chan struct{}),
		start:   time.Now(),
		spin:    spin.New(),
		w:       w,
	}

	go h.loop()

	return h
}

// Close the handler.
func (h *Handler) Close() error {
	h.done <- struct{}{}
	close(h.done)
	close(h.entries)
	return nil
}

// loop for rendering.
func (h *Handler) loop() {
	ticker := time.NewTicker(100 * time.Millisecond)

	for {
		select {
		case e := <-h.entries:
			if h.prev != nil {
				h.render(h.prev, true)
			}
			h.render(e, false)
			h.prev = e
		case <-ticker.C:
			if h.prev != nil {
				h.render(h.prev, false)
			}
			h.spin.Next()
		case <-h.done:
			ticker.Stop()
			if h.prev != nil {
				h.render(h.prev, true)
			}
			return
		}
	}
}

func (h *Handler) render(e *log.Entry, done bool) {
	color := Colors[e.Level]
	level := Strings[e.Level]
	names := e.Fields.Names()

	// delta and spinner
	if done {
		fmt.Fprintf(h.w, "\r     %-7s", time.Since(h.start).Round(time.Millisecond))
	} else {
		fmt.Fprintf(h.w, "\r   %s %-7s", h.spin.Current(), time.Since(h.start).Round(time.Millisecond))
	}

	// message
	fmt.Fprintf(h.w, " %s %s", color(level), color(e.Message))

	// fields
	for _, name := range names {
		v := e.Fields.Get(name)

		if v == "" {
			continue
		}

		fmt.Fprintf(h.w, " %s%s%v", color(name), gray("="), v)
	}

	// newline
	if done {
		fmt.Fprintf(h.w, "\n")
		h.start = time.Now()
	}
}

// HandleLog implements log.Handler.
func (h *Handler) HandleLog(e *log.Entry) error {
	h.entries <- e
	return nil
}
