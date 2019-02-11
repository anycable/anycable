package mruby

import "unsafe"

// #cgo CFLAGS: -Ivendor/mruby/include
// #cgo LDFLAGS: ${SRCDIR}/libmruby.a -lm
// #include <stdlib.h>
// #include "gomruby.h"
import "C"

// Mrb represents a single instance of mruby.
type Mrb struct {
	state *C.mrb_state
}

// ArenaIndex represents the index into the arena portion of the GC.
//
// See ArenaSave for more information.
type ArenaIndex int

// NewMrb creates a new instance of Mrb, representing the state of a single
// Ruby VM.
//
// When you're finished with the VM, clean up all resources it is using
// by calling the Close method.
func NewMrb() *Mrb {
	state := C.mrb_open()

	return &Mrb{
		state: state,
	}
}

// ArenaRestore restores the arena index so the objects between the save and this point
// can be garbage collected in the future.
//
// See ArenaSave for more documentation.
func (m *Mrb) ArenaRestore(idx ArenaIndex) {
	C.mrb_gc_arena_restore(m.state, C.int(idx))
}

// ArenaSave saves the index into the arena.
//
// Restore the arena index later by calling ArenaRestore.
//
// The arena is where objects returned by functions such as LoadString
// are stored. By saving the index and then later restoring it with
// ArenaRestore, these objects can be garbage collected. Otherwise, the
// objects will never be garbage collected.
//
// The recommended usage pattern for memory management is to save
// the arena index prior to any Ruby execution, to turn the resulting
// Ruby value into Go values as you see fit, then to restore the arena
// index so that GC can collect any values.
//
// Of course, when Close() is called, all objects in the arena are
// garbage collected anyways, so if you're only calling mruby for a short
// period of time, you might not have to worry about saving/restoring the
// arena.
func (m *Mrb) ArenaSave() ArenaIndex {
	return ArenaIndex(C.mrb_gc_arena_save(m.state))
}

// EnableGC enables the garbage collector for this mruby instance. It returns
// true if garbage collection was previously disabled.
func (m *Mrb) EnableGC() {
	C._go_enable_gc(m.state)
}

// DisableGC disables the garbage collector for this mruby instance. It returns
// true if it was previously disabled.
func (m *Mrb) DisableGC() {
	C._go_disable_gc(m.state)
}

// LiveObjectCount returns the number of objects that have not been collected (aka, alive).
func (m *Mrb) LiveObjectCount() int {
	return int(C._go_gc_live(m.state))
}

// Class returns the class with the kgiven name and superclass. Note that
// if you call this with a class that doesn't exist, mruby will abort the
// application (like a panic, but not a Go panic).
//
// super can be nil, in which case the Object class will be used.
func (m *Mrb) Class(name string, super *Class) *Class {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))

	var class *C.struct_RClass
	if super == nil {
		class = C.mrb_class_get(m.state, cs)
	} else {
		class = C.mrb_class_get_under(m.state, super.class, cs)
	}

	return newClass(m, class)
}

// Module returns the named module as a *Class. If the module is invalid,
// NameError is triggered within your program and SIGABRT is sent to the
// application.
func (m *Mrb) Module(name string) *Class {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))

	class := C.mrb_module_get(m.state, cs)

	return newClass(m, class)
}

// Close a Mrb, this must be called to properly free resources, and
// should only be called once.
func (m *Mrb) Close() {
	// Delete all the methods from the state
	stateMethodTable.Mutex.Lock()
	delete(stateMethodTable.Map, m.state)
	stateMethodTable.Mutex.Unlock()

	// Close the state
	C.mrb_close(m.state)
}

// ConstDefined checks if the given constant is defined in the scope.
//
// This should be used, for example, before a call to Class, because a
// failure in Class will crash your program (by design). You can retrieve
// the Value of a Class by calling Value().
func (m *Mrb) ConstDefined(name string, scope Value) bool {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))

	scopeV := scope.MrbValue(m).value
	b := C.mrb_const_defined(
		m.state, scopeV, C.mrb_intern_cstr(m.state, cs))
	return C.ushort(b) != 0
}

// FullGC executes a complete GC cycle on the VM.
func (m *Mrb) FullGC() {
	C.mrb_full_gc(m.state)
}

// GetArgs returns all the arguments that were given to the currnetly
// called function (currently on the stack).
func (m *Mrb) GetArgs() []*MrbValue {
	getArgLock.Lock()
	defer getArgLock.Unlock()

	// Clear reset the accumulator to zero length
	getArgAccumulator = make([]C.mrb_value, 0, C._go_get_max_funcall_args())

	// Get all the arguments and put it into our accumulator
	count := C._go_mrb_get_args_all(m.state)

	// Convert those all to values
	values := make([]*MrbValue, count)

	for i := 0; i < int(count); i++ {
		values[i] = newValue(m.state, getArgAccumulator[i])
	}

	return values
}

// IncrementalGC runs an incremental GC step. It is much less expensive
// than a FullGC, but must be called multiple times for GC to actually
// happen.
//
// This function is best called periodically when executing Ruby in
// the VM many times (thousands of times).
func (m *Mrb) IncrementalGC() {
	C.mrb_incremental_gc(m.state)
}

// LoadString loads the given code, executes it, and returns its final
// value that it might return.
func (m *Mrb) LoadString(code string) (*MrbValue, error) {
	cs := C.CString(code)
	defer C.free(unsafe.Pointer(cs))

	value := C._go_mrb_load_string(m.state, cs)
	if exc := checkException(m.state); exc != nil {
		return nil, exc
	}

	return newValue(m.state, value), nil
}

// Run executes the given value, which should be a proc type.
//
// If you're looking to execute code directly a string, look at LoadString.
//
// If self is nil, it is set to the top-level self.
func (m *Mrb) Run(v Value, self Value) (*MrbValue, error) {
	if self == nil {
		self = m.TopSelf()
	}

	mrbV := v.MrbValue(m)
	mrbSelf := self.MrbValue(m)

	proc := C._go_mrb_proc_ptr(mrbV.value)
	value := C.mrb_run(m.state, proc, mrbSelf.value)

	if exc := checkException(m.state); exc != nil {
		return nil, exc
	}

	return newValue(m.state, value), nil
}

// RunWithContext is a context-aware parser (aka, it does not discard state
// between runs). It returns a magic integer that describes the stack in place,
// so that it can be re-used on the next call. This is how local variables can
// traverse ruby parse invocations.
//
// Otherwise, it is very similar in function to Run()
func (m *Mrb) RunWithContext(v Value, self Value, stackKeep int) (int, *MrbValue, error) {
	if self == nil {
		self = m.TopSelf()
	}

	mrbV := v.MrbValue(m)
	mrbSelf := self.MrbValue(m)
	proc := C._go_mrb_proc_ptr(mrbV.value)

	i := C.int(stackKeep)

	value := C._go_mrb_context_run(m.state, proc, mrbSelf.value, &i)

	if exc := checkException(m.state); exc != nil {
		return stackKeep, nil, exc
	}

	return int(i), newValue(m.state, value), nil
}

// Yield yields to a block with the given arguments.
//
// This should be called within the context of a Func.
func (m *Mrb) Yield(block Value, args ...Value) (*MrbValue, error) {
	mrbBlock := block.MrbValue(m)

	var argv []C.mrb_value
	var argvPtr *C.mrb_value

	if len(args) > 0 {
		// Make the raw byte slice to hold our arguments we'll pass to C
		argv = make([]C.mrb_value, len(args))
		for i, arg := range args {
			argv[i] = arg.MrbValue(m).value
		}

		argvPtr = &argv[0]
	}

	result := C._go_mrb_yield_argv(
		m.state,
		mrbBlock.value,
		C.mrb_int(len(argv)),
		argvPtr)

	if exc := checkException(m.state); exc != nil {
		return nil, exc
	}

	return newValue(m.state, result), nil
}

//-------------------------------------------------------------------
// Functions handling defining new classes/modules in the VM
//-------------------------------------------------------------------

// DefineClass defines a new top-level class.
//
// If super is nil, the class will be defined under Object.
func (m *Mrb) DefineClass(name string, super *Class) *Class {
	if super == nil {
		super = m.ObjectClass()
	}

	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))

	return newClass(
		m, C.mrb_define_class(m.state, cs, super.class))
}

// DefineClassUnder defines a new class under another class.
//
// This is, for example, how you would define the World class in
// `Hello::World` where Hello is the "outer" class.
func (m *Mrb) DefineClassUnder(name string, super *Class, outer *Class) *Class {
	if super == nil {
		super = m.ObjectClass()
	}
	if outer == nil {
		outer = m.ObjectClass()
	}

	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))

	return newClass(m, C.mrb_define_class_under(
		m.state, outer.class, cs, super.class))
}

// DefineModule defines a top-level module.
func (m *Mrb) DefineModule(name string) *Class {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	return newClass(m, C.mrb_define_module(m.state, cs))
}

// DefineModuleUnder defines a module under another class/module.
func (m *Mrb) DefineModuleUnder(name string, outer *Class) *Class {
	if outer == nil {
		outer = m.ObjectClass()
	}

	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))

	return newClass(m,
		C.mrb_define_module_under(m.state, outer.class, cs))
}

//-------------------------------------------------------------------
// Functions below return Values or constant Classes
//-------------------------------------------------------------------

// ObjectClass returns the Object top-level class.
func (m *Mrb) ObjectClass() *Class {
	return newClass(m, m.state.object_class)
}

// KernelModule returns the Kernel top-level module.
func (m *Mrb) KernelModule() *Class {
	return newClass(m, m.state.kernel_module)
}

// TopSelf returns the top-level `self` value.
func (m *Mrb) TopSelf() *MrbValue {
	return newValue(m.state, C.mrb_obj_value(unsafe.Pointer(m.state.top_self)))
}

// FalseValue returns a Value for "false"
func (m *Mrb) FalseValue() *MrbValue {
	return newValue(m.state, C.mrb_false_value())
}

// NilValue returns "nil"
func (m *Mrb) NilValue() *MrbValue {
	return newValue(m.state, C.mrb_nil_value())
}

// TrueValue returns a Value for "true"
func (m *Mrb) TrueValue() *MrbValue {
	return newValue(m.state, C.mrb_true_value())
}

// FixnumValue returns a Value for a fixed number.
func (m *Mrb) FixnumValue(v int) *MrbValue {
	return newValue(m.state, C.mrb_fixnum_value(C.mrb_int(v)))
}

// StringValue returns a Value for a string.
func (m *Mrb) StringValue(s string) *MrbValue {
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	return newValue(m.state, C.mrb_str_new_cstr(m.state, cs))
}

func checkException(state *C.mrb_state) error {
	if state.exc == nil {
		return nil
	}

	err := newExceptionValue(state)
	state.exc = nil

	return err
}
