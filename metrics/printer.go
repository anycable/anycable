package metrics

import (
	"log/slog"
	"strings"

	"github.com/anycable/anycable-go/utils"
)

// Printer describes metrics logging interface
type Printer interface {
	Print(snapshot map[string]int64)
}

// BasePrinter simply logs stats as structured log
type BasePrinter struct {
	filter map[string]struct{}
}

// NewBasePrinter returns new base printer struct
func NewBasePrinter(filterList []string) *BasePrinter {
	var filter map[string]struct{}

	if filterList != nil {
		filter = make(map[string]struct{}, len(filterList))
		for _, k := range filterList {
			filter[k] = struct{}{}
		}
	}

	return &BasePrinter{filter: filter}
}

// Run prints a message to the log with metrics logging details
func (p *BasePrinter) Run(interval int) error {
	if p.filter != nil {
		slog.With("context", "metrics").Info("log metrics", "interval", interval, "fields", strings.Join(utils.Keys(p.filter), ","))
	} else {
		slog.With("context", "metrics").Info("log metrics", "interval", interval)
	}
	return nil
}

func (p *BasePrinter) Stop() {
}

// Write prints formatted snapshot to the log
func (p *BasePrinter) Write(m *Metrics) error {
	snapshot := m.IntervalSnapshot()
	p.Print(snapshot)
	return nil
}

// Print logs stats data using global logger with info level
func (p *BasePrinter) Print(snapshot map[string]uint64) {
	fields := make([]interface{}, 0)

	for k, v := range snapshot {
		if p.filter == nil {
			fields = append(fields, k, v)
		} else if _, ok := p.filter[k]; ok {
			fields = append(fields, k, v)
		}
	}

	slog.With("context", "metrics").Info("", fields...)
}
