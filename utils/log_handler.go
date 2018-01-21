package utils

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/apex/log"
)

// colors.
const (
	none   = 0
	red    = 31
	green  = 32
	yellow = 33
	blue   = 34
	gray   = 37
)

// Colors mapping.
var Colors = [...]int{
	log.DebugLevel: gray,
	log.InfoLevel:  blue,
	log.WarnLevel:  yellow,
	log.ErrorLevel: red,
	log.FatalLevel: red,
}

// Strings mapping.
var Strings = [...]string{
	log.DebugLevel: "DEBUG",
	log.InfoLevel:  "INFO",
	log.WarnLevel:  "WARN",
	log.ErrorLevel: "ERROR",
	log.FatalLevel: "FATAL",
}

// Chars mapping.
var Chars = [...]string{
	log.DebugLevel: "D",
	log.InfoLevel:  "I",
	log.WarnLevel:  "W",
	log.ErrorLevel: "E",
	log.FatalLevel: "F",
}

// LogHandler with TTY awareness
type LogHandler struct {
	mu     sync.Mutex
	writer io.Writer
	tty    bool
}

const (
	timeFormat = "2006-01-02T15:04:05.000Z"
)

// HandleLog is a method called by logger to record a log entry
func (h *LogHandler) HandleLog(e *log.Entry) error {
	names := e.Fields.Names()
	ts := time.Now().UTC().Format(timeFormat)

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.tty {
		color := Colors[e.Level]
		level := Strings[e.Level]

		fmt.Fprintf(h.writer, "\033[%dm%6s\033[0m %s", color, level, ts)

		for _, name := range names {
			fmt.Fprintf(h.writer, " \033[%dm%s\033[0m=%v", color, name, e.Fields.Get(name))
		}

		fmt.Fprintf(h.writer, " \033[%dm%-25s\033[0m\n", color, e.Message)

	} else {
		level := Chars[e.Level]

		fmt.Fprintf(h.writer, "%s %s", level, ts)

		for _, name := range names {
			fmt.Fprintf(h.writer, " %s=%v", name, e.Fields.Get(name))
		}

		fmt.Fprintf(h.writer, " %-25s\n", e.Message)
	}

	return nil
}
