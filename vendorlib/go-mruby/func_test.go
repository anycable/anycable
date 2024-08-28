package mruby

import "testing"

func testCallback(m *Mrb, self *MrbValue) (Value, Value) {
	return Int(42), nil
}

func testCallbackResult(t *testing.T, v *MrbValue) {
	if v.Type() != TypeFixnum {
		t.Fatalf("bad type: %d", v.Type())
	}

	if v.Fixnum() != 42 {
		t.Fatalf("bad: %d", v.Fixnum())
	}
}

func testCallbackException(m *Mrb, self *MrbValue) (Value, Value) {
	_, e := m.LoadString(`raise 'Exception'`)
	v := e.(*Exception)
	return nil, v.MrbValue
}
