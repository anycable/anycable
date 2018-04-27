package mruby

// #include "gomruby.h"
import "C"

// Hash represents an MrbValue that is a Hash in Ruby.
//
// A Hash can be obtained by calling the Hash function on MrbValue.
type Hash struct {
	*MrbValue
}

// Delete deletes a key from the hash, returning its existing value,
// or nil if there wasn't a value.
func (h *Hash) Delete(key Value) (*MrbValue, error) {
	keyVal := key.MrbValue(&Mrb{h.state}).value
	result := C.mrb_hash_delete_key(h.state, h.value, keyVal)

	val := newValue(h.state, result)
	if val.Type() == TypeNil {
		val = nil
	}

	return val, nil
}

// Get reads a value from the hash.
func (h *Hash) Get(key Value) (*MrbValue, error) {
	keyVal := key.MrbValue(&Mrb{h.state}).value
	result := C.mrb_hash_get(h.state, h.value, keyVal)
	return newValue(h.state, result), nil
}

// Set sets a value on the hash
func (h *Hash) Set(key, val Value) error {
	keyVal := key.MrbValue(&Mrb{h.state}).value
	valVal := val.MrbValue(&Mrb{h.state}).value
	C.mrb_hash_set(h.state, h.value, keyVal, valVal)
	return nil
}

// Keys returns the array of keys that the Hash has. This is returned
// as an *MrbValue since this is a Ruby array. You can iterate over it as
// you see fit.
func (h *Hash) Keys() (*MrbValue, error) {
	result := C.mrb_hash_keys(h.state, h.value)
	return newValue(h.state, result), nil
}
