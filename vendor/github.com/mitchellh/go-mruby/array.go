package mruby

// #include "gomruby.h"
import "C"

// Array represents an MrbValue that is a Array in Ruby.
//
// A Array can be obtained by calling the Array function on MrbValue.
type Array struct {
	*MrbValue
}

// Len returns the length of the array.
func (v *Array) Len() int {
	return int(C.mrb_ary_len(v.state, v.value))
}

// Get gets an element form the Array by index.
//
// This does not copy the element. This is a pointer/reference directly
// to the element in the array.
func (v *Array) Get(idx int) (*MrbValue, error) {
	result := C.mrb_ary_entry(v.value, C.mrb_int(idx))

	val := newValue(v.state, result)
	if val.Type() == TypeNil {
		val = nil
	}

	return val, nil
}
