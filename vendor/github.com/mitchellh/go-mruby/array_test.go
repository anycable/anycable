package mruby

import (
	"testing"
)

func TestArray(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	value, err := mrb.LoadString(`["foo", "bar", "baz", false]`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	v := value.Array()

	// Len
	if n := v.Len(); n != 4 {
		t.Fatalf("bad: %d", n)
	}

	// Get
	value, err = v.Get(1)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if value.String() != "bar" {
		t.Fatalf("bad: %s", value)
	}

	// Get bool
	value, err = v.Get(3)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if valType := value.Type(); valType != TypeFalse {
		t.Fatalf("bad type: %v", valType)
	}
	if value.String() != "false" {
		t.Fatalf("bad: %s", value)
	}
}
