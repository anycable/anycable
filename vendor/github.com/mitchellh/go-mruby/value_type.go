package mruby

// ValueType is an enum of types that a Value can be and is returned by
// Value.Type().
type ValueType uint32

const (
	// TypeFalse is `false`
	TypeFalse ValueType = iota
	// TypeFree is ?
	TypeFree
	// TypeTrue is `true`
	TypeTrue
	// TypeFixnum is fixnums, or integers for this case.
	TypeFixnum
	// TypeSymbol is for entities in ruby that look like `:this`
	TypeSymbol
	// TypeUndef is a value internal to ruby for uninstantiated vars.
	TypeUndef
	// TypeFloat is any floating point number such as 1.2, etc.
	TypeFloat
	// TypeCptr is a void*
	TypeCptr
	// TypeObject is a standard ruby object, base class of most instantiated objects.
	TypeObject
	// TypeClass is the base class of all classes.
	TypeClass
	// TypeModule is the base class of all Modules.
	TypeModule
	// TypeIClass is ?
	TypeIClass
	// TypeSClass is ?
	TypeSClass
	// TypeProc are procs (concrete block definitons)
	TypeProc
	// TypeArray is []
	TypeArray
	// TypeHash is { }
	TypeHash
	// TypeString is ""
	TypeString
	// TypeRange is (0..x)
	TypeRange
	// TypeException is raised when using the raise keyword
	TypeException
	// TypeFile is for objects of the File class
	TypeFile
	// TypeEnv is for getenv/setenv etc
	TypeEnv
	// TypeData is ?
	TypeData
	// TypeFiber is for members of the Fiber class
	TypeFiber
	// TypeMaxDefine is ?
	TypeMaxDefine
	// TypeNil is nil
	TypeNil ValueType = 0xffffffff
)
