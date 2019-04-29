package tebata

import (
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"sync"
)

// Tebata struct has any status.
type Tebata struct {
	mutex            *sync.Mutex
	signalCh         chan os.Signal
	reservedFunction []functionData
}

type functionData struct {
	function interface{}
	args     []interface{}
}

// New Tebata struct, and start to catch signal.
func New(signals ...os.Signal) *Tebata {
	s := &Tebata{
		mutex:    new(sync.Mutex),
		signalCh: make(chan os.Signal, 1),
	}
	signal.Notify(s.signalCh, signals...)
	go s.listen()
	return s
}

func (s *Tebata) listen() {
	for {
		select {
		case <-s.signalCh:
			s.exec()
		}
	}
}

func (s *Tebata) exec() {
	defer s.mutex.Unlock()
	s.mutex.Lock()
	for _, rf := range s.reservedFunction {
		argsValueOf := reflect.ValueOf(rf.args)
		argsKind := argsValueOf.Kind()
		argsTypeName := argsValueOf.Type().Name()

		switch argsKind {
		case reflect.Slice:
			// Expand argsValue for convert args element from interface{} to reflect.Value
			var argsValue []reflect.Value
			argsInterface := argsValueOf.Interface().([]interface{})
			for _, arg := range argsInterface {
				argsValue = append(argsValue, reflect.ValueOf(arg))
			}

			// Call function
			function := reflect.ValueOf(rf.function)
			function.Call(argsValue)
		default:
			panic(fmt.Sprintf("Invalid function arguments. arguments type: %s", argsTypeName))
		}
	}
}

// Reserve the function to be executed when receiving the Linux signal.
func (s *Tebata) Reserve(function interface{}, args ...interface{}) error {
	defer s.mutex.Unlock()
	s.mutex.Lock()
	if reflect.ValueOf(function).Kind() != reflect.Func {
		return fmt.Errorf("Invalid \"function\" argument.\n Expect Type: func")
	}
	if reflect.ValueOf(args).Kind() != reflect.Slice {
		return fmt.Errorf("Invalid \"args\" argument.\n Expect Type: slice")
	}

	s.reservedFunction = append(
		s.reservedFunction,
		functionData{
			function,
			convertInterfaceSlice(args),
		},
	)

	return nil
}

func convertInterfaceSlice(args interface{}) (convertedSlice []interface{}) {
	a := reflect.ValueOf(args)
	length := a.Len()
	convertedSlice = make([]interface{}, length)

	for i := 0; i < length; i++ {
		convertedSlice[i] = a.Index(i).Interface()
	}

	return convertedSlice
}
