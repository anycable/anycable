// +build darwin,mrb linux,mrb

package mrb

import (
	"fmt"
	"io/ioutil"
	"sync"

	nanoid "github.com/matoous/go-nanoid"
	"github.com/mitchellh/go-mruby"
)

// Supported returns true iff mruby scripting is available
func Supported() bool {
	return true
}

// Engine represents one running mruby VM
type Engine struct {
	VM *mruby.Mrb
	mu sync.Mutex
}

var (
	defaultEngine     *Engine
	defaultEngineSync sync.Mutex
)

// NewEngine builds new mruby VM and return new engine
func NewEngine() *Engine {
	return &Engine{VM: mruby.NewMrb()}
}

// DefaultEngine returns a default mruby engine
func DefaultEngine() *Engine {
	defaultEngineSync.Lock()
	defer defaultEngineSync.Unlock()

	if defaultEngine == nil {
		defaultEngine = NewEngine()
	}

	return defaultEngine
}

// LoadFile loads, parses and eval Ruby file within a vm
func (engine *Engine) LoadFile(path string) error {
	contents, err := ioutil.ReadFile(path)

	if err != nil {
		return err
	}

	return engine.LoadString(string(contents))
}

// LoadString loads, parses and eval Ruby code within a vm
func (engine *Engine) LoadString(contents string) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	ctx := mruby.NewCompileContext(engine.VM)
	defer ctx.Close()

	filename, err := nanoid.Nanoid()

	if err != nil {
		return err
	}

	ctx.SetFilename(fmt.Sprintf("%s.rb", filename))

	parser := mruby.NewParser(engine.VM)
	defer parser.Close()

	if _, err = parser.Parse(contents, ctx); err != nil {
		return err
	}

	parsed := parser.GenerateCode()

	if _, err = engine.VM.Run(parsed, nil); err != nil {
		return err
	}

	return nil
}

// Eval runs arbitrary code within a vm
func (engine *Engine) Eval(code string) (*mruby.MrbValue, error) {
	return engine.VM.LoadString(code)
}
