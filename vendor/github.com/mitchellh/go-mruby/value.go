package mruby

import (
	"fmt"
	"strconv"
	"strings"
	"unsafe"
)

// #include <stdlib.h>
// #include "gomruby.h"
import "C"

// Value is an interface that should be implemented by anything that can
// be represents as an mruby value.
type Value interface {
	MrbValue(*Mrb) *MrbValue
}

// Int is the basic ruby Integer type.
type Int int

// NilType is the object representation of NilClass
type NilType [0]byte

// String is objects of the type String.
type String string

// Nil is a constant that can be used as a Nil Value
var Nil NilType

// MrbValue is a "value" internally in mruby. A "value" is what mruby calls
// basically anything in Ruby: a class, an object (instance), a variable,
// etc.
type MrbValue struct {
	value C.mrb_value
	state *C.mrb_state
}

func init() {
	Nil = [0]byte{}
}

// SetInstanceVariable sets an instance variable on this value.
func (v *MrbValue) SetInstanceVariable(variable string, value *MrbValue) {
	cs := C.CString(variable)
	defer C.free(unsafe.Pointer(cs))
	C._go_mrb_iv_set(v.state, v.value, C.mrb_intern_cstr(v.state, cs), value.value)
}

// GetInstanceVariable gets an instance variable on this value.
func (v *MrbValue) GetInstanceVariable(variable string) *MrbValue {
	cs := C.CString(variable)
	defer C.free(unsafe.Pointer(cs))
	return newValue(v.state, C._go_mrb_iv_get(v.state, v.value, C.mrb_intern_cstr(v.state, cs)))
}

// Call calls a method with the given name and arguments on this
// value.
func (v *MrbValue) Call(method string, args ...Value) (*MrbValue, error) {
	return v.call(method, args, nil)
}

// CallBlock is the same as call except that it expects the last
// argument to be a Proc that will be passed into the function call.
// It is an error if args is empty or if there is no block on the end.
func (v *MrbValue) CallBlock(method string, args ...Value) (*MrbValue, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("args must be non-empty and have a proc at the end")
	}

	n := len(args)
	return v.call(method, args[:n-1], args[n-1])
}

func (v *MrbValue) call(method string, args []Value, block Value) (*MrbValue, error) {
	var argv []C.mrb_value
	var argvPtr *C.mrb_value

	mrb := &Mrb{v.state}

	if len(args) > 0 {
		// Make the raw byte slice to hold our arguments we'll pass to C
		argv = make([]C.mrb_value, len(args))
		for i, arg := range args {
			argv[i] = arg.MrbValue(mrb).value
		}

		argvPtr = &argv[0]
	}

	var blockV *C.mrb_value
	if block != nil {
		val := block.MrbValue(mrb).value
		blockV = &val
	}

	cs := C.CString(method)
	defer C.free(unsafe.Pointer(cs))

	// If we have a block, we have to call a separate function to
	// pass a block in. Otherwise, we just call it directly.
	result := C._go_mrb_call(
		v.state,
		v.value,
		C.mrb_intern_cstr(v.state, cs),
		C.mrb_int(len(argv)),
		argvPtr,
		blockV)

	if exc := checkException(v.state); exc != nil {
		return nil, exc
	}

	return newValue(v.state, result), nil
}

// IsDead tells you if an object has been collected by the GC or not.
func (v *MrbValue) IsDead() bool {
	return C.ushort(C._go_isdead(v.state, v.value)) != 0
}

// MrbValue so that *MrbValue implements the "Value" interface.
func (v *MrbValue) MrbValue(*Mrb) *MrbValue {
	return v
}

// Mrb returns the Mrb state for this value.
func (v *MrbValue) Mrb() *Mrb {
	return &Mrb{v.state}
}

// GCProtect protects this value from being garbage collected.
func (v *MrbValue) GCProtect() {
	C.mrb_gc_protect(v.state, v.value)
}

// SetProcTargetClass sets the target class where a proc will be executed
// when this value is a proc.
func (v *MrbValue) SetProcTargetClass(c *Class) {
	proc := C._go_mrb_proc_ptr(v.value)
	proc.target_class = c.class
}

// Type returns the ValueType of the MrbValue. See the constants table.
func (v *MrbValue) Type() ValueType {
	if C._go_mrb_nil_p(v.value) == 1 {
		return TypeNil
	}

	return ValueType(C._go_mrb_type(v.value))
}

// Exception is a special type of value that represents an error
// and implements the Error interface.
type Exception struct {
	*MrbValue
	File      string
	Line      int
	Message   string
	Backtrace []string
}

func (e *Exception) Error() string {
	return e.Message
}

func (e *Exception) String() string {
	return e.Message
}

//-------------------------------------------------------------------
// Type conversions to Go types
//-------------------------------------------------------------------

// Array returns the Array value of this value. If the Type of the MrbValue
// is not a TypeArray, then this will panic. If the MrbValue has a
// `to_a` function, you must call that manually prior to calling this
// method.
func (v *MrbValue) Array() *Array {
	return &Array{v}
}

// Fixnum returns the numeric value of this object if the Type() is
// TypeFixnum. Calling this with any other type will result in undefined
// behavior.
func (v *MrbValue) Fixnum() int {
	return int(C._go_mrb_fixnum(v.value))
}

// Float returns the numeric value of this object if the Type() is
// TypeFloat. Calling this with any other type will result in undefined
// behavior.
func (v *MrbValue) Float() float64 {
	return float64(C._go_mrb_float(v.value))
}

// Hash returns the Hash value of this value. If the Type of the MrbValue
// is not a ValueTypeHash, then this will panic. If the MrbValue has a
// `to_h` function, you must call that manually prior to calling this
// method.
func (v *MrbValue) Hash() *Hash {
	return &Hash{v}
}

// String returns the "to_s" result of this value.
func (v *MrbValue) String() string {
	value := C.mrb_obj_as_string(v.state, v.value)
	result := C.GoString(C.mrb_string_value_ptr(v.state, value))
	return result
}

// Class returns the *Class of a value.
func (v *MrbValue) Class() *Class {
	mrb := &Mrb{v.state}
	return newClass(mrb, C.mrb_class(v.state, v.value))
}

// SingletonClass returns the singleton class (a class isolated just for the
// scope of the object) for the given value.
func (v *MrbValue) SingletonClass() *Class {
	mrb := &Mrb{v.state}
	sclass := C._go_mrb_class_ptr(C.mrb_singleton_class(v.state, v.value))
	return newClass(mrb, sclass)
}

//-------------------------------------------------------------------
// Native Go types implementing the Value interface
//-------------------------------------------------------------------

// MrbValue returns the native MRB value
func (i Int) MrbValue(m *Mrb) *MrbValue {
	return m.FixnumValue(int(i))
}

// MrbValue returns the native MRB value
func (NilType) MrbValue(m *Mrb) *MrbValue {
	return m.NilValue()
}

// MrbValue returns the native MRB value
func (s String) MrbValue(m *Mrb) *MrbValue {
	return m.StringValue(string(s))
}

//-------------------------------------------------------------------
// Internal Functions
//-------------------------------------------------------------------

func newExceptionValue(s *C.mrb_state) *Exception {
	if s.exc == nil {
		panic("exception value init without exception")
	}

	arenaIndex := C.mrb_gc_arena_save(s)
	defer C.mrb_gc_arena_restore(s, C.int(arenaIndex))

	// Convert the RObject* to an mrb_value
	value := C.mrb_obj_value(unsafe.Pointer(s.exc))

	// Retrieve and convert backtrace to []string (avoiding reflection in Decode)
	var backtrace []string
	mrbBacktrace := newValue(s, C.mrb_exc_backtrace(s, value)).Array()
	for i := 0; i < mrbBacktrace.Len(); i++ {
		ln, _ := mrbBacktrace.Get(i)
		backtrace = append(backtrace, ln.String())
	}

	// Extract file + line from first backtrace line
	file := "Unknown"
	line := 0
	if len(backtrace) > 0 {
		fileAndLine := strings.Split(backtrace[0], ":")
		if len(fileAndLine) >= 2 {
			file = fileAndLine[0]
			line, _ = strconv.Atoi(fileAndLine[1])
		}
	}

	result := newValue(s, value)
	return &Exception{
		MrbValue:  result,
		Message:   result.String(),
		File:      file,
		Line:      line,
		Backtrace: backtrace,
	}
}

func newValue(s *C.mrb_state, v C.mrb_value) *MrbValue {
	return &MrbValue{
		state: s,
		value: v,
	}
}
