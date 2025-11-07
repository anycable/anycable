//go:build (darwin && mrb) || (linux && mrb)

package metrics

import (
	"fmt"
	"log/slog"

	"github.com/anycable/anycable-go/mrb"
	"github.com/mitchellh/go-mruby"
)

// RubyPrinter contains refs to mruby vm and code
type RubyPrinter struct {
	path      string
	mrbModule *mruby.MrbValue
	engine    *mrb.Engine

	log *slog.Logger
}

// NewCustomPrinter generates log formatter from the provided (as path)
// Ruby script
func NewCustomPrinter(path string, l *slog.Logger) (*RubyPrinter, error) {
	return &RubyPrinter{path: path, log: l}, nil
}

// Run initializes the Ruby VM
func (p *RubyPrinter) Run(interval int) error {
	p.engine = mrb.DefaultEngine()

	if err := p.engine.LoadFile(p.path); err != nil {
		return err
	}

	mod := p.engine.VM.Module("MetricsFormatter")

	p.mrbModule = mod.MrbValue(p.engine.VM)

	p.log.Info(fmt.Sprintf("Log metrics every %ds using a custom Ruby formatter from %s", interval, p.path))

	return nil
}

func (p *RubyPrinter) Stop() {
}

// Write prints formatted snapshot to the log
func (p *RubyPrinter) Write(m *Metrics) error {
	snapshot := m.IntervalSnapshot()
	p.Print(snapshot)
	return nil
}

// Print calls Ruby script to format the output and prints it to the log
func (p *RubyPrinter) Print(snapshot map[string]uint64) {
	rhash, _ := p.engine.VM.LoadString("{}")

	hash := rhash.Hash()

	for k, v := range snapshot {
		hash.Set(mruby.String(k), mruby.Int(v))
	}

	result, err := p.mrbModule.Call("call", rhash)

	if err != nil {
		p.log.Error("mruby call failed", "error", err)
		return
	}

	p.log.Info(result.String())
}
