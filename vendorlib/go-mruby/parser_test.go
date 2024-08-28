package mruby

import (
	"testing"
)

func TestParserGenerateCode(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	p := NewParser(mrb)
	defer p.Close()

	warns, err := p.Parse(`"foo"`, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if warns != nil {
		t.Fatalf("warnings: %v", warns)
	}

	proc := p.GenerateCode()
	result, err := mrb.Run(proc, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result.String() != "foo" {
		t.Fatalf("bad: %s", result.String())
	}
}

func TestParserParse(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	p := NewParser(mrb)
	defer p.Close()

	warns, err := p.Parse(`"foo"`, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if warns != nil {
		t.Fatalf("warnings: %v", warns)
	}
}

func TestParserParse_error(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	p := NewParser(mrb)
	defer p.Close()

	_, err := p.Parse(`def foo`, nil)
	if err == nil {
		t.Fatal("should have errors")
	}
}

func TestParserError_error(t *testing.T) {
	var _ error = new(ParserError)
}
