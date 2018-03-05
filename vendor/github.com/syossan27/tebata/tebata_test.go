package tebata

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"
	"testing"
)

func TestNew(t *testing.T) {
	// TODO: Write Test!
}

func TestStatus_Reserve(t *testing.T) {
	// TODO: Write Test!
}

func TestStatus_exec(t *testing.T) {
	done := make(chan int, 1)

	stdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	s := New(syscall.SIGINT, syscall.SIGTERM)
	s.Reserve(
		func(first, second int, done chan int) {
			fmt.Print(strconv.Itoa(first + second))
			done <- 1
		},
		1, 2, done,
	)

	s.signalCh <- os.Interrupt
	<-done

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = stdout

	if buf.Len() == 0 {
		t.Error("Output empty")
	}

	if buf.String() != "3" {
		t.Error("Invalid output")
	}
}

func TestStatus_exec_race_check(t *testing.T) {
	done := make(chan int, 1)

	stdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	s1 := New(syscall.SIGINT, syscall.SIGTERM)
	s1.Reserve(
		func(first, second int, done chan int) {
			fmt.Print(strconv.Itoa(first + second))
			done <- 1
		},
		1, 2, done,
	)

	s2 := New(syscall.SIGINT, syscall.SIGTERM)
	s2.Reserve(
		func(first, second int, done chan int) {
			fmt.Print(strconv.Itoa(first + second))
			done <- 1
		},
		1, 2, done,
	)

	s1.signalCh <- os.Interrupt
	s2.signalCh <- os.Interrupt
	<-done

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = stdout

	if buf.Len() == 0 {
		t.Error("Output empty")
	}
}
