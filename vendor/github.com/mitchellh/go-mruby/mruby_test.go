package mruby

import (
	"fmt"
	"reflect"
	"testing"
)

func TestNewMrb(t *testing.T) {
	mrb := NewMrb()
	mrb.Close()
}

func TestMrbArena(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	idx := mrb.ArenaSave()
	mrb.ArenaRestore(idx)
}

func TestMrbModule(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	module := mrb.Module("Kernel")
	if module == nil {
		t.Fatal("module was nil and should not be")
	}
}

func TestMrbClass(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	var class *Class
	class = mrb.Class("Object", nil)
	if class == nil {
		t.Fatal("class should not be nil")
	}

	mrb.DefineClass("Hello", mrb.ObjectClass())
	class = mrb.Class("Hello", mrb.ObjectClass())
	if class == nil {
		t.Fatal("class should not be nil")
	}
}

func TestMrbConstDefined(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	if !mrb.ConstDefined("Object", mrb.ObjectClass()) {
		t.Fatal("Object should be defined")
	}

	mrb.DefineClass("Hello", mrb.ObjectClass())
	if !mrb.ConstDefined("Hello", mrb.ObjectClass()) {
		t.Fatal("Hello should be defined")
	}
}

func TestMrbDefineClass(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	mrb.DefineClass("Hello", mrb.ObjectClass())
	_, err := mrb.LoadString("Hello")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	mrb.DefineClass("World", nil)
	_, err = mrb.LoadString("World")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestMrbDefineClass_methodException(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	cb := func(m *Mrb, self *MrbValue) (Value, Value) {
		v, err := m.LoadString(`raise "exception"`)
		if err != nil {
			exc := err.(*Exception)
			return nil, exc.MrbValue
		}

		return v, nil
	}

	class := mrb.DefineClass("Hello", mrb.ObjectClass())
	class.DefineClassMethod("foo", cb, ArgsNone())
	_, err := mrb.LoadString(`Hello.foo`)
	if err == nil {
		t.Fatal("should error")
	}
}

func TestMrbDefineClassUnder(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	// Define an outer
	hello := mrb.DefineClass("Hello", mrb.ObjectClass())
	_, err := mrb.LoadString("Hello")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Inner
	mrb.DefineClassUnder("World", nil, hello)
	_, err = mrb.LoadString("Hello::World")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Inner defaults
	mrb.DefineClassUnder("Another", nil, nil)
	_, err = mrb.LoadString("Another")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestMrbDefineModule(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	mrb.DefineModule("Hello")
	_, err := mrb.LoadString("Hello")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestMrbDefineModuleUnder(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	// Define an outer
	hello := mrb.DefineModule("Hello")
	_, err := mrb.LoadString("Hello")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Inner
	mrb.DefineModuleUnder("World", hello)
	_, err = mrb.LoadString("Hello::World")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Inner defaults
	mrb.DefineModuleUnder("Another", nil)
	_, err = mrb.LoadString("Another")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestMrbFixnumValue(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	value := mrb.FixnumValue(42)
	if value.Type() != TypeFixnum {
		t.Fatalf("should be fixnum")
	}
}

func TestMrbFullGC(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	ai := mrb.ArenaSave()
	value := mrb.StringValue("foo")
	if value.IsDead() {
		t.Fatal("should not be dead")
	}

	mrb.ArenaRestore(ai)
	mrb.FullGC()
	if !value.IsDead() {
		t.Fatal("should be dead")
	}
}

type testcase struct {
	args   string
	types  []ValueType
	result []string
}

func TestMrbGetArgs(t *testing.T) {
	cases := []testcase{
		{
			`("foo")`,
			[]ValueType{TypeString},
			[]string{`"foo"`},
		},

		{
			`(true)`,
			[]ValueType{TypeTrue},
			[]string{`true`},
		},

		{
			`(Hello)`,
			[]ValueType{TypeClass},
			[]string{`Hello`},
		},

		{
			`() { }`,
			[]ValueType{TypeProc},
			nil,
		},

		{
			`(Hello, "bar", true)`,
			[]ValueType{TypeClass, TypeString, TypeTrue},
			[]string{`Hello`, `"bar"`, "true"},
		},

		{
			`("bar", true) {}`,
			[]ValueType{TypeString, TypeTrue, TypeProc},
			nil,
		},
	}

	// lots of this effort is centered around testing multithreaded behavior.

	for i := 0; i < 1000; i++ {

		errChan := make(chan error, len(cases))

		for _, tc := range cases {
			go func(tc testcase) {
				var actual []*MrbValue
				testFunc := func(m *Mrb, self *MrbValue) (Value, Value) {
					actual = m.GetArgs()
					return self, nil
				}

				mrb := NewMrb()
				defer mrb.Close()
				class := mrb.DefineClass("Hello", mrb.ObjectClass())
				class.DefineClassMethod("test", testFunc, ArgsAny())
				_, err := mrb.LoadString(fmt.Sprintf("Hello.test%s", tc.args))
				if err != nil {
					errChan <- fmt.Errorf("err: %s", err)
					return
				}

				if tc.result != nil {
					if len(actual) != len(tc.result) {
						errChan <- fmt.Errorf("%s: expected %d, got %d",
							tc.args, len(tc.result), len(actual))
						return
					}
				}

				actualStrings := make([]string, len(actual))
				actualTypes := make([]ValueType, len(actual))
				for i, v := range actual {
					str, err := v.Call("inspect")
					if err != nil {
						t.Fatalf("err: %s", err)
					}

					actualStrings[i] = str.String()
					actualTypes[i] = v.Type()
				}

				if !reflect.DeepEqual(actualTypes, tc.types) {
					errChan <- fmt.Errorf("code: %s\nexpected: %#v\nactual: %#v",
						tc.args, tc.types, actualTypes)
					return
				}

				if tc.result != nil {
					if !reflect.DeepEqual(actualStrings, tc.result) {
						errChan <- fmt.Errorf("expected: %#v\nactual: %#v",
							tc.result, actualStrings)
						return
					}
				}

				errChan <- nil
			}(tc)
		}

		for range cases {
			if err := <-errChan; err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestMrbLoadString(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	value, err := mrb.LoadString(`"HELLO"`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if value == nil {
		t.Fatalf("should have value")
	}
}

func TestMrbLoadString_twice(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	value, err := mrb.LoadString(`"HELLO"`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if value == nil {
		t.Fatalf("should have value")
	}

	value, err = mrb.LoadString(`"WORLD"`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if value.String() != "WORLD" {
		t.Fatalf("bad: %s", value)
	}
}

func TestMrbLoadStringException(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	_, err := mrb.LoadString(`raise "An exception"`)

	if err == nil {
		t.Fatal("exception expected")
	}

	value, err := mrb.LoadString(`"test"`)
	if err != nil {
		t.Fatal("exception should have been cleared")
	}

	if value.String() != "test" {
		t.Fatal("bad test value returned")
	}
}

func TestMrbRaise(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	cb := func(m *Mrb, self *MrbValue) (Value, Value) {
		return nil, m.GetArgs()[0]
	}

	class := mrb.DefineClass("Hello", mrb.ObjectClass())
	class.DefineClassMethod("foo", cb, ArgsReq(1))
	_, err := mrb.LoadString(`Hello.foo(ArgumentError.new("ouch"))`)
	if err == nil {
		t.Fatal("should have error")
	}
	if err.Error() != "ouch" {
		t.Fatalf("bad: %s", err)
	}
}

func TestMrbYield(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	cb := func(m *Mrb, self *MrbValue) (Value, Value) {
		result, err := m.Yield(m.GetArgs()[0], Int(12), Int(30))
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		return result, nil
	}

	class := mrb.DefineClass("Hello", mrb.ObjectClass())
	class.DefineClassMethod("foo", cb, ArgsBlock())
	value, err := mrb.LoadString(`Hello.foo { |a, b| a + b }`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if value.Fixnum() != 42 {
		t.Fatalf("bad: %s", value)
	}
}

func TestMrbYieldException(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	cb := func(m *Mrb, self *MrbValue) (Value, Value) {
		result, err := m.Yield(m.GetArgs()[0])
		if err != nil {
			exc := err.(*Exception)
			return nil, exc.MrbValue
		}

		return result, nil
	}

	class := mrb.DefineClass("Hello", mrb.ObjectClass())
	class.DefineClassMethod("foo", cb, ArgsBlock())
	_, err := mrb.LoadString(`Hello.foo { raise "exception" }`)
	if err == nil {
		t.Fatal("should error")
	}

	_, err = mrb.LoadString(`Hello.foo { 1 }`)
	if err != nil {
		t.Fatal("exception should have been cleared")
	}
}

func TestMrbRun(t *testing.T) {
	mrb := NewMrb()
	defer mrb.Close()

	parser := NewParser(mrb)
	defer parser.Close()
	context := NewCompileContext(mrb)
	defer context.Close()

	parser.Parse(`
		if $do_raise
			raise "exception"
		else
			"rval"
		end`,
		context,
	)

	proc := parser.GenerateCode()

	// Enable proc exception raising & verify
	mrb.LoadString(`$do_raise = true`)
	_, err := mrb.Run(proc, nil)

	if err == nil {
		t.Fatalf("expected exception, %#v", err)
	}

	// Disable proc exception raising
	// If we still have an exception, it wasn't cleared from the previous invocation.
	mrb.LoadString(`$do_raise = false`)
	rval, err := mrb.Run(proc, nil)
	if err != nil {
		t.Fatalf("unexpected exception, %#v", err)
	}

	if rval.String() != "rval" {
		t.Fatalf("expected return value 'rval', got %#v", rval)
	}

	parser.Parse(`a = 10`, context)
	proc = parser.GenerateCode()

	stackKeep, ret, err := mrb.RunWithContext(proc, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	if stackKeep != 2 {
		t.Fatalf("stack value was %d not 2; some variables may not have been captured", stackKeep)
	}

	parser.Parse(`a`, context)
	proc = parser.GenerateCode()

	stackKeep, ret, err = mrb.RunWithContext(proc, nil, stackKeep)
	if err != nil {
		t.Fatal(err)
	}

	if ret.String() != "10" {
		t.Fatalf("Captured variable was not expected value: was %q", ret.String())
	}
}

func TestMrbDefineMethodConcurrent(t *testing.T) {
	concurrency := 100
	numFuncs := 100

	cb := func(m *Mrb, self *MrbValue) (Value, Value) {
		return m.GetArgs()[0], nil
	}

	syncChan := make(chan struct{}, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			mrb := NewMrb()
			defer mrb.Close()
			for i := 0; i < numFuncs; i++ {
				mrb.TopSelf().SingletonClass().DefineMethod(fmt.Sprintf("test%d", i), cb, ArgsAny())
			}

			syncChan <- struct{}{}
		}()
	}

	for i := 0; i < concurrency; i++ {
		<-syncChan
	}
}

func TestMrbStackedException(t *testing.T) {
	var testClass *Class

	createException := func(m *Mrb, msg string) Value {
		val, err := m.Class("Exception", nil).New(String(msg))
		if err != nil {
			panic(fmt.Sprintf("could not construct exception for return: %v", err))
		}

		return val
	}

	testFunc := func(m *Mrb, self *MrbValue) (Value, Value) {
		args := m.GetArgs()

		t, err := testClass.New()
		if err != nil {
			return nil, createException(m, err.Error())
		}

		argv := []Value{}
		for _, arg := range args {
			argv = append(argv, Value(arg))
		}
		v, err := t.Call("dotest!", argv...)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		return v, nil
	}

	doTestFunc := func(m *Mrb, self *MrbValue) (Value, Value) {
		err := createException(m, "Fail us!")
		return nil, err
	}

	mrb := NewMrb()

	testClass = mrb.DefineClass("TestClass", nil)
	testClass.DefineMethod("dotest!", doTestFunc, ArgsReq(0)|ArgsOpt(3))

	mrb.TopSelf().SingletonClass().DefineMethod("test", testFunc, ArgsReq(0)|ArgsOpt(3))

	_, err := mrb.LoadString("test")
	if err == nil {
		t.Fatal("No exception when one was expected")
		return
	}

	mrb.Close()
	mrb = NewMrb()

	evalFunc := func(m *Mrb, self *MrbValue) (Value, Value) {
		arg := m.GetArgs()[0]
		result, err := self.CallBlock("instance_eval", arg)
		if err != nil {
			return result, createException(m, err.Error())
		}

		return result, nil
	}

	mrb.TopSelf().SingletonClass().DefineMethod("myeval", evalFunc, ArgsBlock())

	result, err := mrb.LoadString("myeval { raise 'foo' }")
	if err == nil {
		t.Fatal("did not error")
		return
	}

	if result != nil {
		t.Fatal("result was not cleared")
		return
	}

	mrb.Close()
}
