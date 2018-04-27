package mruby

import (
	"reflect"
	"testing"
)

func TestExceptionString_afterClose(t *testing.T) {
	mrb := NewMrb()
	_, err := mrb.LoadString(`clearly a syntax error`)
	mrb.Close()
	// This panics before the bug fix that this test tests
	err.Error()
}

func TestExceptionBacktrace(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	parser := NewParser(mrb)
	defer parser.Close()
	context := NewCompileContext(mrb)
	context.SetFilename("hello.rb")
	defer context.Close()

	parser.Parse(`
				def do_error
					raise "Exception"
				end

				def hop1
					do_error
				end

				def hop2
					hop1
				end

				hop2
			`, context)

	proc := parser.GenerateCode()
	_, err := mrb.Run(proc, nil)
	if err == nil {
		t.Fatalf("expected exception")
	}

	exc := err.(*Exception)
	if exc.Message != "Exception" {
		t.Fatalf("bad exception message: %s", exc.Message)
	}

	if exc.File != "hello.rb" {
		t.Fatalf("bad file: %s", exc.File)
	}

	if exc.Line != 3 {
		t.Fatalf("bad line: %d", exc.Line)
	}

	if len(exc.Backtrace) != 4 {
		t.Fatalf("bad backtrace: %#v", exc.Backtrace)
	}
}

func TestMrbValueCall(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	value, err := mrb.LoadString(`"foo"`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	_, err = value.Call("some_function_that_doesnt_exist")
	if err == nil {
		t.Fatalf("expected exception")
	}

	result, err := value.Call("==", String("foo"))
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result.Type() != TypeTrue {
		t.Fatalf("bad type")
	}
}

func TestMrbValueCallBlock(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	value, err := mrb.LoadString(`"foo"`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	block, err := mrb.LoadString(`Proc.new { |_| "bar" }`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	result, err := value.CallBlock("gsub", String("foo"), block)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result.Type() != TypeString {
		t.Fatalf("bad type")
	}
	if result.String() != "bar" {
		t.Fatalf("bad: %s", result)
	}
}

func TestMrbValueValue(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	falseV := mrb.FalseValue()
	if falseV.MrbValue(mrb) != falseV {
		t.Fatal("should be the same")
	}
}

func TestMrbValueValue_impl(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	var _ Value = mrb.FalseValue()
}

func TestMrbValueFixnum(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	value, err := mrb.LoadString("42")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if value.Fixnum() != 42 {
		t.Fatalf("bad fixnum")
	}
}

func TestMrbValueString(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	value, err := mrb.LoadString(`"foo"`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if value.String() != "foo" {
		t.Fatalf("bad string")
	}
}

func TestMrbValueType(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	cases := []struct {
		Input    string
		Expected ValueType
	}{
		{
			`false`,
			TypeFalse,
		},
		// TypeFree - Type of value after GC collection
		{
			`true`,
			TypeTrue,
		},
		{
			`1`,
			TypeFixnum,
		},
		{
			`:test`,
			TypeSymbol,
		},
		// TypeUndef - Internal value used by mruby for undefined things (instance vars etc)
		// These all seem to get converted to exceptions before hitting userland
		{
			`1.1`,
			TypeFloat,
		},
		// TypeCptr
		{
			`Object.new`,
			TypeObject,
		},
		{
			`Object`,
			TypeClass,
		},
		{
			`module T; end; T`,
			TypeModule,
		},
		// TypeIClass
		// TypeSClass
		{
			`Proc.new { 1 }`,
			TypeProc,
		},
		{
			`[]`,
			TypeArray,
		},
		{
			`{}`,
			TypeHash,
		},
		{
			`"string"`,
			TypeString,
		},
		{
			`1..2`,
			TypeRange,
		},
		{
			`Exception.new`,
			TypeException,
		},
		// TypeFile
		// TypeEnv
		// TypeData
		// TypeFiber
		// TypeMaxDefine
		{
			`nil`,
			TypeNil,
		},
	}

	for _, c := range cases {
		r, err := mrb.LoadString(c.Input)
		if err != nil {
			t.Fatalf("loadstring failed for case %#v: %s", c, err)
		}
		if cType := r.Type(); cType != c.Expected {
			t.Fatalf("bad type: got %v, expected %v", cType, c.Expected)
		}
	}
}

func TestIntMrbValue(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	var value Value = Int(42)
	v := value.MrbValue(mrb)
	if v.Fixnum() != 42 {
		t.Fatalf("bad value")
	}
}

func TestStringMrbValue(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	var value Value = String("foo")
	v := value.MrbValue(mrb)
	if v.String() != "foo" {
		t.Fatalf("bad value")
	}
}

func TestValueClass(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	val, err := mrb.ObjectClass().New()
	if err != nil {
		t.Fatalf("Error constructing object of type Object: %v", err)
	}

	if !reflect.DeepEqual(val.Class(), mrb.ObjectClass()) {
		t.Fatal("Class of value was not equivalent to constructed class")
	}
}

func TestValueSingletonClass(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	fn := func(m *Mrb, self *MrbValue) (Value, Value) {
		args := m.GetArgs()
		return Int(args[0].Fixnum() + args[1].Fixnum()), nil
	}

	mrb.TopSelf().SingletonClass().DefineMethod("add", fn, ArgsReq(2))

	result, err := mrb.LoadString(`add(46, 2)`)
	if err != nil {
		t.Fatalf("Error parsing ruby code: %v", err)
	}

	if result.String() != "48" {
		t.Fatalf("Result %q was not equal to the target value of 48", result.String())
	}
}
