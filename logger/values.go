package logger

import (
	"fmt"
	"log/slog"
)

const (
	maxValueLength = 100
)

type compactValue[T string | []byte] struct {
	val T
}

func (c *compactValue[T]) LogValue() slog.Value {
	return slog.StringValue(c.String())
}

func (c *compactValue[T]) String() string {
	val := string(c.val)

	if len(val) > maxValueLength {
		return fmt.Sprintf("%s...(%d)", val[:maxValueLength], len(val)-maxValueLength)
	}

	return val
}

// CompactValue wraps any scalar value to show it in log truncated
func CompactValue[T string | []byte](v T) *compactValue[T] {
	return &compactValue[T]{val: v}
}

func CompactValues[T string | []byte, S []T](v S) []*compactValue[T] {
	res := make([]*compactValue[T], len(v))
	for i, val := range v {
		res[i] = CompactValue(val)
	}

	return res
}

type compactAny struct {
	val interface{}
}

func (c *compactAny) String() string {
	val := fmt.Sprintf("%+v", c.val)

	if len(val) > maxValueLength {
		return fmt.Sprintf("%s...(%d)", val[:maxValueLength], len(val)-maxValueLength)
	}

	return val
}

func (c *compactAny) LogValue() slog.Value {
	return slog.StringValue(c.String())
}

// CompactAny wraps any value to show it in log truncated
func CompactAny(val interface{}) *compactAny {
	return &compactAny{val}
}
