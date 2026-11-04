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

	t.Run("sse comment pings flag", func(t *testing.T) {
		c, err, _ := NewConfigFromCLI([]string{"", "--ignore-config-path", "--sse_comment_pings"})
		require.NoError(t, err)

		assert.True(t, c.SSE.CommentPings)
		assert.Equal(t, 15, c.SSE.PingInterval)
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

func TestUnknownConfigKeys(t *testing.T) {
	writeConfig := func(t *testing.T, conf string) string {
		path := filepath.Join(t.TempDir(), "anycable.toml")
		require.NoError(t, os.WriteFile(path, []byte(conf), 0600))
		return path
	}

	t.Run("valid configuration", func(t *testing.T) {
		path := writeConfig(t, "secret = \"s3cr3t\"\n\n[sse]\nenabled = true\n")

		c, err, _ := NewConfigFromCLI([]string{"", "--config-path=" + path, "--strict-config"})
		require.NoError(t, err)

		assert.True(t, c.SSE.Enabled)
		assert.Empty(t, c.UnknownConfigKeys)
	})

	t.Run("unknown keys are recorded", func(t *testing.T) {
		path := writeConfig(t, "hots = \"0.0.0.0\"\n\n[redis]\nurll = \"redis://redis:6379\"\n")

		c, err, _ := NewConfigFromCLI([]string{"", "--config-path=" + path})
		require.NoError(t, err)

		assert.Equal(t, []string{"hots", "redis.urll"}, c.UnknownConfigKeys)
	})

	t.Run("strict mode fails on unknown keys", func(t *testing.T) {
		path := writeConfig(t, "hots = \"0.0.0.0\"\n\n[redis]\nurll = \"redis://redis:6379\"\n")

		_, err, _ := NewConfigFromCLI([]string{"", "--config-path=" + path, "--strict-config"})
		require.Error(t, err)

		assert.Contains(t, err.Error(), "unknown keys in configuration file")
		assert.Contains(t, err.Error(), "hots, redis.urll")
	})

	t.Run("strict mode via env var", func(t *testing.T) {
		t.Setenv("ANYCABLE_STRICT_CONFIG", "true")

		path := writeConfig(t, "[redis]\nurll = \"redis://redis:6379\"\n")

		_, err, _ := NewConfigFromCLI([]string{"", "--config-path=" + path})
		require.Error(t, err)

		assert.Contains(t, err.Error(), "redis.urll")
	})

	t.Run("printed configuration has no unknown keys", func(t *testing.T) {
		c, err, _ := NewConfigFromCLI([]string{"", "--ignore-config-path", "--metrics_http=/metrics", "--metrics_tags=env:test", "--redis_url=redis://redis:6379", "--sse", "--embed_nats"})
		require.NoError(t, err)

		path := writeConfig(t, c.ToToml())

		c, err, _ = NewConfigFromCLI([]string{"", "--config-path=" + path, "--strict-config"})
		require.NoError(t, err)

		assert.Equal(t, "/metrics", c.Metrics.HTTP)
		assert.Empty(t, c.UnknownConfigKeys)
	})
}
