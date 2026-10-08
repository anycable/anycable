package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTagsArg(t *testing.T) {
	expected := map[string]string{"env": "dev", "rev": "1.1"}

	headers := parseTags("env:dev,rev:1.1")
	assert.Equal(t, expected, headers)
}

func TestCommentPingIntervals(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		c, err, _ := NewConfigFromCLI([]string{"", "--ignore-config-path"})
		require.NoError(t, err)

		assert.Equal(t, 15, c.SSE.PingInterval)
		assert.Equal(t, 15, c.DS.SSEPingInterval)
	})

	t.Run("inherits custom global ping interval", func(t *testing.T) {
		c, err, _ := NewConfigFromCLI([]string{"", "--ignore-config-path", "--ping_interval=30"})
		require.NoError(t, err)

		assert.Equal(t, 30, c.SSE.PingInterval)
		assert.Equal(t, 30, c.DS.SSEPingInterval)
	})

	t.Run("component-specific values take precedence", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "anycable.toml")
		conf := "[app]\nping_interval = 30\n\n[sse]\nping_interval = 20\n\n[ds]\nsse_ping_interval = 25\n"
		require.NoError(t, os.WriteFile(path, []byte(conf), 0600))

		c, err, _ := NewConfigFromCLI([]string{"", "--config-path=" + path})
		require.NoError(t, err)

		assert.Equal(t, 30, c.App.PingInterval)
		assert.Equal(t, 20, c.SSE.PingInterval)
		assert.Equal(t, 25, c.DS.SSEPingInterval)
	})
}
