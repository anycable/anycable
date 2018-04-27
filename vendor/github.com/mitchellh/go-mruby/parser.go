package mruby

import (
	"bytes"
	"fmt"
	"unsafe"
)

// #include <stdlib.h>
// #include "gomruby.h"
import "C"

// Parser is a parser for Ruby code.
type Parser struct {
	code   string
	mrb    *Mrb
	parser *C.struct_mrb_parser_state
}

// NewParser initializes the resources for a parser.
//
// Make sure to Close the parser when you're done with it.
func NewParser(m *Mrb) *Parser {
	p := C.mrb_parser_new(m.state)

	// Set capture_errors to true so we don't go just printing things
	// out to stdout.
	C._go_mrb_parser_set_capture_errors(p, 1)

	return &Parser{
		mrb:    m,
		parser: p,
	}
}

// Close releases any resources associated with the parser.
func (p *Parser) Close() {
	C.mrb_parser_free(p.parser)

	// Empty out the code so the other string can get GCd
	p.code = ""
}

// GenerateCode takes all the internal parser state and generates
// executable Ruby code, returning the callable proc.
func (p *Parser) GenerateCode() *MrbValue {
	proc := C.mrb_generate_code(p.mrb.state, p.parser)
	return newValue(p.mrb.state, C.mrb_obj_value(unsafe.Pointer(proc)))
}

// Parse parses the code in the given context, and returns any warnings
// or errors from parsing.
//
// The CompileContext can be nil to not set a context.
func (p *Parser) Parse(code string, c *CompileContext) ([]*ParserMessage, error) {
	// We set p.code so that the string doesn't get garbage collected
	var s *C.char = C.CString(code)
	p.code = code
	p.parser.s = s
	p.parser.send = C._go_mrb_calc_send(s)

	var ctx *C.mrbc_context
	if c != nil {
		ctx = c.ctx
	}
	C.mrb_parser_parse(p.parser, ctx)

	var warnings []*ParserMessage
	if p.parser.nwarn > 0 {
		nwarn := int(p.parser.nwarn)
		warnings = make([]*ParserMessage, nwarn)
		for i := 0; i < nwarn; i++ {
			msg := p.parser.warn_buffer[i]

			warnings[i] = &ParserMessage{
				Col:     int(msg.column),
				Line:    int(msg.lineno),
				Message: C.GoString(msg.message),
			}
		}
	}

	if p.parser.nerr > 0 {
		nerr := int(p.parser.nerr)
		errors := make([]*ParserMessage, nerr)
		for i := 0; i < nerr; i++ {
			msg := p.parser.error_buffer[i]

			errors[i] = &ParserMessage{
				Col:     int(msg.column),
				Line:    int(msg.lineno),
				Message: C.GoString(msg.message),
			}
		}

		return warnings, &ParserError{Errors: errors}
	}

	return warnings, nil
}

// ParserMessage represents a message from parsing code: a warning or
// error.
type ParserMessage struct {
	Col     int
	Line    int
	Message string
}

// ParserError is an error from the parser.
type ParserError struct {
	Errors []*ParserMessage
}

func (p ParserError) Error() string {
	return p.String()
}

func (p ParserError) String() string {
	var buf bytes.Buffer
	buf.WriteString("Ruby parse error!\n\n")
	for _, e := range p.Errors {
		buf.WriteString(fmt.Sprintf("line %d:%d: %s\n", e.Line, e.Col, e.Message))
	}

	return buf.String()
}
