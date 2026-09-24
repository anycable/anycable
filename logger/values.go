package logger

import (
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
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

const (
	filteredMask = "***"
	// Values up to this length are masked completely
	fullMaskMaxLength = 5
	// Values up to this length keep only the first and the last characters visible
	shortMaskMaxLength = 8
)

type filteredURL struct {
	val string
}

func (f *filteredURL) String() string {
	u, err := url.Parse(f.val)

	if err != nil {
		return "[invalid url]"
	}

	// We build user info manually, since url.URL escapes the mask characters
	var userinfo string

	if u.User != nil {
		userinfo = MaskValue(u.User.Username())

		if password, ok := u.User.Password(); ok {
			userinfo += ":" + MaskValue(password)
		}

		u.User = nil
	}

	if u.RawQuery != "" {
		params := strings.Split(u.RawQuery, "&")

		for i, param := range params {
			if key, value, ok := strings.Cut(param, "="); ok {
				params[i] = key + "=" + MaskValue(value)
			}
		}

		u.RawQuery = strings.Join(params, "&")
	}

	if userinfo != "" {
		return strings.Replace(u.String(), "//", "//"+userinfo+"@", 1)
	}

	return u.String()
}

func (f *filteredURL) LogValue() slog.Value {
	return slog.StringValue(f.String())
}

// FilteredURL wraps a URL to show it in log with masked query parameter values and user info
func FilteredURL(val string) *filteredURL {
	return &filteredURL{val}
}

// MaskValue hides the value, keeping only the first and the last characters visible (if the value is long enough)
func MaskValue(val string) string {
	if val == "" {
		return ""
	}

	visible := 2

	if len(val) <= fullMaskMaxLength {
		return filteredMask
	} else if len(val) <= shortMaskMaxLength {
		visible = 1
	}

	return val[:visible] + filteredMask + val[len(val)-visible:]
}

type filteredMap[T any] struct {
	val map[string]T
}

func (f *filteredMap[T]) LogValue() slog.Value {
	keys := make([]string, 0, len(f.val))

	for k := range f.val {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	return slog.AnyValue(keys)
}

// FilteredMap wraps a map to show only its keys in log
func FilteredMap[T any](val map[string]T) *filteredMap[T] {
	return &filteredMap[T]{val}
}
