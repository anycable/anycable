package logfmt_test

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/apex/log"
	"github.com/apex/log/handlers/logfmt"
)

func init() {
	log.Now = func() time.Time {
		return time.Unix(0, 0).UTC()
	}
}

func Test(t *testing.T) {
	var buf bytes.Buffer

	log.SetHandler(logfmt.New(&buf))
	log.WithField("user", "tj").WithField("id", "123").Info("hello")
	log.Info("world")
	log.Error("boom")

	expected := `timestamp=1970-01-01T00:00:00Z level=info message=hello id=123 user=tj
timestamp=1970-01-01T00:00:00Z level=info message=world
timestamp=1970-01-01T00:00:00Z level=error message=boom
`

	assert.Equal(t, expected, buf.String())
}

func Benchmark(b *testing.B) {
	log.SetHandler(logfmt.New(ioutil.Discard))
	ctx := log.WithField("user", "tj").WithField("id", "123")

	for i := 0; i < b.N; i++ {
		ctx.Info("hello")
	}
}
