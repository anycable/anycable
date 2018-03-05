package tebata

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"sync"
)

type status struct {
	mutex            *sync.Mutex
	signalCh         chan os.Signal
	ReservedFunction []functionData
}

type functionData struct {
	function interface{}
	args     []interface{}
}

func New(signals ...os.Signal) *status {
	s := &status{
		mutex:    new(sync.Mutex),
		signalCh: make(chan os.Signal, 1),
	}
	signal.Notify(s.signalCh, signals...)
	go s.listen()
	return s
}

func (s *status) Reserve(function interface{}, args ...interface{}) error {
	defer s.mutex.Unlock()
	s.mutex.Lock()
	if reflect.ValueOf(function).Kind() != reflect.Func {
		return errors.New(
			fmt.Sprintf("Invalid \"function\" argument.\n Expect Type: func"),
		)
	}
	if reflect.ValueOf(args).Kind() != reflect.Slice {
		return errors.New(
			fmt.Sprintf("Invalid \"args\" argument.\n Expect Type: slice"),
		)
	}

	s.ReservedFunction = append(
		s.ReservedFunction,
		functionData{
			function,
			convertInterfaceSlice(args),
		},
	)

	return nil
}

func (s *status) exec() {
	defer s.mutex.Unlock()
	s.mutex.Lock()
	for _, rf := range s.ReservedFunction {
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

func convertInterfaceSlice(args interface{}) (convertedSlice []interface{}) {
	a := reflect.ValueOf(args)
	length := a.Len()
	convertedSlice = make([]interface{}, length)

	for i := 0; i < length; i++ {
		convertedSlice[i] = a.Index(i).Interface()
	}

	return convertedSlice
}

func (s *status) listen() {
	for {
		select {
		case <-s.signalCh:
			s.exec()
			return
		}
	}
}
