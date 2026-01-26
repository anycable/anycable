package api

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_NewConfig(t *testing.T) {
	config := NewConfig()

	assert.Equal(t, defaultAPIPath, config.Path)
	assert.Equal(t, 0, config.Port)
	assert.Equal(t, "", config.Host)
	assert.Equal(t, "", config.Secret)
	assert.Equal(t, "", config.SecretBase)
	assert.False(t, config.AddCORSHeaders)
	assert.Equal(t, "", config.CORSHosts)
}

func TestConfig_IsSecured(t *testing.T) {
	t.Run("returns false when no secret configured", func(t *testing.T) {
		config := NewConfig()
		assert.False(t, config.IsSecured())
	})

	t.Run("returns true when Secret is set", func(t *testing.T) {
		config := NewConfig()
		config.Secret = "test-secret"
		assert.True(t, config.IsSecured())
	})

	t.Run("returns true when SecretBase is set", func(t *testing.T) {
		config := NewConfig()
		config.SecretBase = "test-secret-base"
		assert.True(t, config.IsSecured())
	})
}

func TestConfig_DeriveSecret(t *testing.T) {
	t.Run("does nothing when Secret is already set", func(t *testing.T) {
		config := NewConfig()
		config.Secret = "existing-secret"
		config.SecretBase = "some-base"

		err := config.DeriveSecret()
		require.NoError(t, err)

		assert.Equal(t, "existing-secret", config.Secret)
	})

	t.Run("does nothing when SecretBase is empty", func(t *testing.T) {
		config := NewConfig()

		err := config.DeriveSecret()
		require.NoError(t, err)

		assert.Equal(t, "", config.Secret)
	})

	t.Run("derives secret from SecretBase", func(t *testing.T) {
		config := NewConfig()
		config.SecretBase = "qwerty"

		err := config.DeriveSecret()
		require.NoError(t, err)

		assert.NotEmpty(t, config.Secret)
		// The derived secret should be deterministic
		expectedSecret := config.Secret

		config2 := NewConfig()
		config2.SecretBase = "qwerty"
		err = config2.DeriveSecret()
		require.NoError(t, err)

		assert.Equal(t, expectedSecret, config2.Secret)
	})

	t.Run("different SecretBase produces different secrets", func(t *testing.T) {
		config1 := NewConfig()
		config1.SecretBase = "secret1"
		err := config1.DeriveSecret()
		require.NoError(t, err)

		config2 := NewConfig()
		config2.SecretBase = "secret2"
		err = config2.DeriveSecret()
		require.NoError(t, err)

		assert.NotEqual(t, config1.Secret, config2.Secret)
	})
}

func TestConfig_ToToml(t *testing.T) {
	config := NewConfig()
	config.Host = "0.0.0.0"
	config.Port = 8081
	config.Path = "/api"
	config.Secret = "my-secret"
	config.AddCORSHeaders = true

	tomlStr := config.ToToml()

	assert.Contains(t, tomlStr, "host = \"0.0.0.0\"")
	assert.Contains(t, tomlStr, "port = 8081")
	assert.Contains(t, tomlStr, "path = \"/api\"")
	assert.Contains(t, tomlStr, "secret = \"my-secret\"")
	assert.Contains(t, tomlStr, "cors_headers = true")

	// Round-trip test
	config2 := NewConfig()
	_, err := toml.Decode(tomlStr, &config2)
	require.NoError(t, err)

	assert.Equal(t, config.Host, config2.Host)
	assert.Equal(t, config.Port, config2.Port)
	assert.Equal(t, config.Path, config2.Path)
	assert.Equal(t, config.Secret, config2.Secret)
	assert.Equal(t, config.AddCORSHeaders, config2.AddCORSHeaders)
}

func TestConfig_ToToml_Defaults(t *testing.T) {
	config := NewConfig()

	tomlStr := config.ToToml()

	// Should have commented out defaults
	assert.Contains(t, tomlStr, "# host = \"localhost\"")
	assert.Contains(t, tomlStr, "port = 0")
	assert.Contains(t, tomlStr, "# secret = \"\"")
	assert.Contains(t, tomlStr, "# cors_headers = false")
}
