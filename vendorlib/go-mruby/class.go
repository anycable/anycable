package mruby

import "unsafe"

// #include <stdlib.h>
// #include "gomruby.h"
import "C"

// Class is a class in mruby. To obtain a Class, use DefineClass or
// one of the variants on the Mrb structure.
type Class struct {
	class *C.struct_RClass
	mrb   *Mrb
}

// DefineClassMethod defines a class-level method on the given class.
func (c *Class) DefineClassMethod(name string, cb Func, as ArgSpec) {
	insertMethod(c.mrb.state, c.class.c, name, cb)

	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))

	C.mrb_define_class_method(
		c.mrb.state,
		c.class,
		cs,
		C._go_mrb_func_t(),
		C.mrb_aspec(as))
}

// DefineConst defines a constant within this class.
func (c *Class) DefineConst(name string, value Value) {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))

	C.mrb_define_const(
		c.mrb.state, c.class, cs, value.MrbValue(c.mrb).value)
}

// DefineMethod defines an instance method on the class.
func (c *Class) DefineMethod(name string, cb Func, as ArgSpec) {
	insertMethod(c.mrb.state, c.class, name, cb)

	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))

	C.mrb_define_method(
		c.mrb.state,
		c.class,
		cs,
		C._go_mrb_func_t(),
		C.mrb_aspec(as))
}

// MrbValue returns a *Value for this Class. *Values are sometimes required
// as arguments where classes should be valid.
func (c *Class) MrbValue(m *Mrb) *MrbValue {
	return newValue(c.mrb.state, C.mrb_obj_value(unsafe.Pointer(c.class)))
}

// New instantiates the class with the given args.
func (c *Class) New(args ...Value) (*MrbValue, error) {
	var argv []C.mrb_value
	var argvPtr *C.mrb_value
	if len(args) > 0 {
		// Make the raw byte slice to hold our arguments we'll pass to C
		argv = make([]C.mrb_value, len(args))
		for i, arg := range args {
			argv[i] = arg.MrbValue(c.mrb).value
		}

		argvPtr = &argv[0]
	}

	result := C.mrb_obj_new(c.mrb.state, c.class, C.mrb_int(len(argv)), argvPtr)
	if exc := checkException(c.mrb.state); exc != nil {
		return nil, exc
	}

	return newValue(c.mrb.state, result), nil
}

func newClass(mrb *Mrb, c *C.struct_RClass) *Class {
	return &Class{
		class: c,
		mrb:   mrb,
	}
}
