package json_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/apex/log"
	"github.com/apex/log/handlers/json"
)

func init() {
	log.Now = func() time.Time {
		return time.Unix(0, 0).UTC()
	}
}

func Test(t *testing.T) {
	var buf bytes.Buffer

	log.SetHandler(json.New(&buf))
	log.WithField("user", "tj").WithField("id", "123").Info("hello")
	log.Info("world")
	log.Error("boom")

	expected := `{"fields":{"id":"123","user":"tj"},"level":"info","timestamp":"1970-01-01T00:00:00Z","message":"hello"}
{"fields":{},"level":"info","timestamp":"1970-01-01T00:00:00Z","message":"world"}
{"fields":{},"level":"error","timestamp":"1970-01-01T00:00:00Z","message":"boom"}
`

	assert.Equal(t, expected, buf.String())
}
