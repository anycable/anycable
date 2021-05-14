// +build darwin,mrb linux,mrb

package metrics

import (
	"github.com/anycable/anycable-go/mrb"
	"github.com/apex/log"
	"github.com/mitchellh/go-mruby"
)

// RubyPrinter contains refs to mruby vm and code
type RubyPrinter struct {
	path      string
	mrbModule *mruby.MrbValue
	engine    *mrb.Engine
}

// NewCustomPrinter generates log formatter from the provided (as path)
// Ruby script
func NewCustomPrinter(path string) (*RubyPrinter, error) {
	return &RubyPrinter{path: path}, nil
}

// Run initializes the Ruby VM
func (p *RubyPrinter) Run(interval int) error {
	p.engine = mrb.DefaultEngine()

	if err := p.engine.LoadFile(p.path); err != nil {
		return err
	}

	mod := p.engine.VM.Module("MetricsFormatter")

	p.mrbModule = mod.MrbValue(p.engine.VM)

	log.WithField("context", "metrics").Infof("Log metrics every %ds using a custom Ruby formatter from %s", interval, p.path)

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
func (printer *RubyPrinter) Print(snapshot map[string]uint64) {
	rhash, _ := printer.engine.VM.LoadString("{}")

	hash := rhash.Hash()

	for k, v := range snapshot {
		hash.Set(mruby.String(k), mruby.Int(v))
	}

	result, err := printer.mrbModule.Call("call", rhash)

	if err != nil {
		log.WithField("context", "metrics").Error(err.Error())
		return
	}

	log.Info(result.String())
}
