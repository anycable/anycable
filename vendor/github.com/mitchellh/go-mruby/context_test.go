package mruby

import (
	"testing"
)

func TestCompileContextFilename(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	ctx := NewCompileContext(mrb)
	defer ctx.Close()

	if ctx.Filename() != "" {
		t.Fatalf("bad filename: %s", ctx.Filename())
	}

	ctx.SetFilename("foo")

	if ctx.Filename() != "foo" {
		t.Fatalf("bad filename: %s", ctx.Filename())
	}
}
