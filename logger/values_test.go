package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompactValue_string(t *testing.T) {
	shortvalue := "log-no-long"
	assert.Equal(t, shortvalue, CompactValue(shortvalue).LogValue().String())

	longvalue := strings.Repeat("log-long", 50)

	truncated := CompactValue(longvalue).LogValue().String()
	assert.Equal(t, "log-longlog-l", truncated[:13])
	assert.Len(t, truncated, maxValueLength+8)
}

func TestCompactValue_bytes(t *testing.T) {
	shortvalue := []byte("log-no-long")
	assert.Equal(t, "log-no-long", CompactValue(shortvalue).LogValue().String())

	longvalue := []byte(strings.Repeat("log-long", 50))

	truncated := CompactValue(longvalue).LogValue().String()
	assert.Equal(t, "log-longlog-l", truncated[:13])
	assert.Len(t, truncated, maxValueLength+8)
}

func TestCompactValues_string(t *testing.T) {
	values := []string{
		"log-no-long",
		strings.Repeat("log-long", 50),
	}

	compacts := CompactValues(values)

	assert.Equal(t, "log-no-long", compacts[0].LogValue().String())

	truncated := compacts[1].LogValue().String()
	assert.Equal(t, "log-longlog-l", truncated[:13])
	assert.Len(t, truncated, maxValueLength+8)
}

func TestCompactValues_bytes(t *testing.T) {
	values := [][]byte{
		[]byte("log-no-long"),
		[]byte(strings.Repeat("log-long", 50)),
	}

	any := slog.Any("t", CompactValues(values)).String()
	assert.Contains(t, any, "log-no-long")

	compacts := CompactValues(values)

	assert.Equal(t, "log-no-long", compacts[0].LogValue().String())

	truncated := compacts[1].LogValue().String()
	assert.Equal(t, "log-longlog-l", truncated[:13])
	assert.Len(t, truncated, maxValueLength+8)
}

func TestCompactAny(t *testing.T) {
	value := struct{ val string }{strings.Repeat("log-long", 50)}
	compact := CompactAny(value)

	logValue := compact.LogValue()
	str := logValue.String()

	assert.Equal(t, "{val:log-long", str[:13])
	assert.Len(t, str, maxValueLength+8)
}

func TestWithJSONHandler(t *testing.T) {
	buf := bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	str := "short"
	longstr := strings.Repeat("long", 500)

	logger.Debug("test", "b", CompactValue([]byte(str)), "l", CompactValues([]string{str, longstr}))
	logged := buf.String()

	assert.Contains(t, logged, str)
	assert.Less(t, len(logged), 300)
}
