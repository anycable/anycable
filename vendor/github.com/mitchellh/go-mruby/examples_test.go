package mruby

import (
	"fmt"
)

func Example_CustomFunction() {
	mrb := NewMrb()
	defer mrb.Close()

	// Our custom function we'll expose to Ruby
	addFunc := func(m *Mrb, self *MrbValue) (Value, Value) {
		args := m.GetArgs()
		return Int(args[0].Fixnum() + args[1].Fixnum()), nil
	}

	// Lets define a custom class and a class method we can call.
	class := mrb.DefineClass("Example", nil)
	class.DefineClassMethod("add", addFunc, ArgsReq(2))

	// Let's call it and inspect the result
	result, err := mrb.LoadString(`Example.add(12, 30)`)
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("Result: %s\n", result.String())
	// Output:
	// Result: 42
}

func Example_Decode() {
	mrb := NewMrb()
	defer mrb.Close()

	// Our custom function we'll expose to Ruby
	var logData interface{}
	logFunc := func(m *Mrb, self *MrbValue) (Value, Value) {
		args := m.GetArgs()
		if err := Decode(&logData, args[0]); err != nil {
			panic(err)
		}

		return nil, nil
	}

	// Lets define a custom class and a class method we can call.
	class := mrb.DefineClass("Example", nil)
	class.DefineClassMethod("log", logFunc, ArgsReq(1))

	// Let's call it and inspect the result
	if _, err := mrb.LoadString(`Example.log({"foo" => "bar"})`); err != nil {
		panic(err.Error())
	}

	fmt.Printf("Result: %v\n", logData)
	// Output:
	// Result: map[foo:bar]
}

func Example_SimulateFiles() {
	mrb := NewMrb()
	defer mrb.Close()

	ctx1 := NewCompileContext(mrb)
	defer ctx1.Close()
	ctx1.SetFilename("foo.rb")

	ctx2 := NewCompileContext(mrb)
	defer ctx2.Close()
	ctx2.SetFilename("bar.rb")

	parser := NewParser(mrb)
	defer parser.Close()

	if _, err := parser.Parse("def foo; bar; end", ctx1); err != nil {
		panic(err.Error())
	}
	code1 := parser.GenerateCode()

	if _, err := parser.Parse("def bar; 42; end", ctx2); err != nil {
		panic(err.Error())
	}
	code2 := parser.GenerateCode()

	if _, err := mrb.Run(code1, nil); err != nil {
		panic(err.Error())
	}

	if _, err := mrb.Run(code2, nil); err != nil {
		panic(err.Error())
	}

	result, err := mrb.LoadString("foo")
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("Result: %s\n", result)
	// Output:
	// Result: 42
}
