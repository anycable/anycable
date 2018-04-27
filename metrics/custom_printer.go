// +build darwin,cgo linux,cgo

package metrics

import (
	"github.com/anycable/anycable-go/mrb"
	"github.com/apex/log"
	"github.com/mitchellh/go-mruby"
)

// RubyPrinter contains refs to mruby vm and code
type RubyPrinter struct {
	mrbModule *mruby.MrbValue
	engine    *mrb.Engine
}

// NewCustomPrinter generates log formatter from the provided (as path)
// Ruby script
func NewCustomPrinter(path string) (*RubyPrinter, error) {
	// return nil, errors.New("Not supported")

	engine := mrb.DefaultEngine()

	if err := engine.LoadFile(path); err != nil {
		return nil, err
	}

	mod := engine.VM.Module("MetricsHandler")

	modValue := mod.MrbValue(engine.VM)

	return &RubyPrinter{mrbModule: modValue, engine: engine}, nil
}

// Print calls Ruby script to format the output and prints it to the log
func (printer *RubyPrinter) Print(snapshot map[string]int64) {
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
