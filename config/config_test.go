package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadConfigFromString(t *testing.T, contents string) *Config {
	t.Helper()

	path := filepath.Join(t.TempDir(), "anycable.toml")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0600))

	config := NewConfig()
	config.ConfigFilePath = path

	require.NoError(t, config.LoadFromFile())

	return &config
}

func TestLoadFromFileUnknownKeys(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		config := loadConfigFromString(t, "secret = \"s3cr3t\"\n\n[redis]\nurl = \"redis://redis:6379\"\n\n[metrics.tags]\nenv = \"test\"\n")

		assert.Equal(t, "s3cr3t", config.Secret)
		assert.Equal(t, "redis://redis:6379", config.Redis.URL)
		assert.Equal(t, map[string]string{"env": "test"}, config.Metrics.Tags)
		assert.Empty(t, config.UnknownConfigKeys)
	})

	t.Run("unknown top-level key", func(t *testing.T) {
		config := loadConfigFromString(t, "secret = \"s3cr3t\"\nsecert = \"typo\"\n")

		assert.Equal(t, "s3cr3t", config.Secret)
		assert.Equal(t, []string{"secert"}, config.UnknownConfigKeys)
	})

	t.Run("unknown nested key", func(t *testing.T) {
		config := loadConfigFromString(t, "[redis]\nurll = \"redis://redis:6379\"\n\n[server]\nport = 8081\nhots = \"0.0.0.0\"\n")

		assert.Equal(t, 8081, config.Server.Port)
		assert.Equal(t, []string{"redis.urll", "server.hots"}, config.UnknownConfigKeys)
	})

	t.Run("unknown table", func(t *testing.T) {
		config := loadConfigFromString(t, "[redis_broadcast]\nstream = \"test\"\ngroup = \"test\"\n\n[sse]\nenabled = true\n")

		assert.True(t, config.SSE.Enabled)
		assert.Equal(t, []string{"redis_broadcast"}, config.UnknownConfigKeys)
	})
}

func TestToTomlRoundTrip(t *testing.T) {
	defaults := NewConfig()

	t.Run("generated configuration", func(t *testing.T) {
		config := loadConfigFromString(t, defaults.ToToml())

		assert.Empty(t, config.UnknownConfigKeys)
	})

	t.Run("with commented out settings enabled", func(t *testing.T) {
		config := loadConfigFromString(t, uncommentSettings(defaults.ToToml()))

		assert.Empty(t, config.UnknownConfigKeys)
	})
}

var (
	settingRx          = regexp.MustCompile(`^([a-z0-9_.]+) = `)
	commentedSettingRx = regexp.MustCompile(`^# ([a-z0-9_.]+) = `)
)

// uncommentSettings activates commented out settings (e.g., "# port = 8080"),
// so we can check that all the keys mentioned in the generated configuration are known.
// Settings already defined in the same table are kept commented out.
func uncommentSettings(contents string) string {
	lines := strings.Split(contents, "\n")
	defined := map[string]bool{}
	table := ""

	for _, line := range lines {
		if strings.HasPrefix(line, "[") {
			table = line
		} else if m := settingRx.FindStringSubmatch(line); m != nil {
			defined[table+m[1]] = true
		}
	}

	table = ""

	for i, line := range lines {
		if strings.HasPrefix(line, "[") {
			table = line
		} else if m := commentedSettingRx.FindStringSubmatch(line); m != nil && !defined[table+m[1]] {
			defined[table+m[1]] = true
			lines[i] = strings.TrimPrefix(line, "# ")
		}
	}

	return strings.Join(lines, "\n")
}
