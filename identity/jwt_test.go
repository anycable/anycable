package identity

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/anycable/anycable-go/common"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTIdentifierIdentify(t *testing.T) {
	secret := "ruby-to-go"
	algo := defaultJWTAlgo
	ids := "{\"user_id\":\"15\"}"

	config := NewJWTConfig(secret)
	subject := NewJWTIdentifier(&config, slog.Default())

	t.Run("with valid token passed as query param", func(t *testing.T) {
		token := jwt.NewWithClaims(algo, jwt.MapClaims{
			"ext": ids,
			"exp": time.Now().Local().Add(time.Hour * time.Duration(1)).Unix(),
		})

		tokenString, err := token.SignedString([]byte(secret))

		require.Nil(t, err)

		env := common.NewSessionEnv(fmt.Sprintf("ws://demo.anycable.io/cable?jid=%s", tokenString), nil)

		res, err := subject.Identify("12", env)

		require.Nil(t, err)
		require.NotNil(t, res)
		assert.Equal(t, ids, res.Identifier)
		assert.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{`{"type":"welcome","sid":"12"}`}, res.Transmissions)
	})

	t.Run("with valid token passed as a header", func(t *testing.T) {
		token := jwt.NewWithClaims(algo, jwt.MapClaims{
			"ext": ids,
			"exp": time.Now().Local().Add(time.Hour * time.Duration(1)).Unix(),
		})

		tokenString, err := token.SignedString([]byte(secret))

		require.Nil(t, err)

		env := common.NewSessionEnv("ws://demo.anycable.io/cable", &map[string]string{"x-jid": tokenString})

		res, err := subject.Identify("12", env)

		require.Nil(t, err)
		require.NotNil(t, res)
		assert.Equal(t, ids, res.Identifier)
		assert.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{`{"type":"welcome","sid":"12"}`}, res.Transmissions)
	})

	t.Run("with invalid token", func(t *testing.T) {
		tokenString := "secret-token-not-a-jwt-at-all"
		env := common.NewSessionEnv(fmt.Sprintf("ws://demo.anycable.io/cable?jid=%s", tokenString), nil)

		res, err := subject.Identify("12", env)

		require.Nil(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "", res.Identifier)
		assert.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{"{\"type\":\"disconnect\",\"reason\":\"unauthorized\",\"reconnect\":false}"}, res.Transmissions)
	})

	t.Run("with invalid algo", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
			"ext": ids,
			"exp": time.Now().Local().Add(time.Hour * time.Duration(1)).Unix(),
		})

		tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

		require.Nil(t, err)

		env := common.NewSessionEnv(fmt.Sprintf("ws://demo.anycable.io/cable?jid=%s", tokenString), nil)

		res, err := subject.Identify("12", env)

		require.Nil(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "", res.Identifier)
		assert.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{"{\"type\":\"disconnect\",\"reason\":\"unauthorized\",\"reconnect\":false}"}, res.Transmissions)
	})

	t.Run("with invalid secret", func(t *testing.T) {
		token := jwt.NewWithClaims(algo, jwt.MapClaims{
			"ext": ids,
			"exp": time.Now().Local().Add(time.Hour * time.Duration(1)).Unix(),
		})

		tokenString, err := token.SignedString([]byte("not-a-valid-secret"))

		require.Nil(t, err)

		env := common.NewSessionEnv(fmt.Sprintf("ws://demo.anycable.io/cable?jid=%s", tokenString), nil)

		res, err := subject.Identify("12", env)

		require.Nil(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "", res.Identifier)
		assert.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{"{\"type\":\"disconnect\",\"reason\":\"unauthorized\",\"reconnect\":false}"}, res.Transmissions)
	})

	t.Run("when token expired", func(t *testing.T) {
		token := jwt.NewWithClaims(algo, jwt.MapClaims{
			"ext": ids,
			"exp": time.Now().Local().Add(-time.Hour * time.Duration(1)).Unix(),
		})

		tokenString, err := token.SignedString([]byte(secret))

		require.Nil(t, err)

		env := common.NewSessionEnv("ws://demo.anycable.io/cable", &map[string]string{"x-jid": tokenString})

		res, err := subject.Identify("12", env)

		require.Nil(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "", res.Identifier)
		assert.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{"{\"type\":\"disconnect\",\"reason\":\"token_expired\",\"reconnect\":false}"}, res.Transmissions)
	})

	t.Run("when token is missing and not required", func(t *testing.T) {
		env := common.NewSessionEnv("ws://demo.anycable.io/cable", nil)

		res, err := subject.Identify("12", env)

		assert.Nil(t, err)
		assert.Nil(t, res)
	})

	t.Run("when token is missing and required", func(t *testing.T) {
		config := NewJWTConfig(secret)
		config.Force = true

		enforced := NewJWTIdentifier(&config, slog.Default())

		env := common.NewSessionEnv("ws://demo.anycable.io/cable", nil)

		res, err := enforced.Identify("12", env)

		require.Nil(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "", res.Identifier)
		assert.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{"{\"type\":\"disconnect\",\"reason\":\"unauthorized\",\"reconnect\":false}"}, res.Transmissions)
	})
}

func TestConfig__ToToml(t *testing.T) {
	conf := NewJWTConfig("jwt-secret")
	conf.Force = false
	conf.Param = "token"

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "param = \"token\"")
	assert.Contains(t, tomlStr, "secret = \"jwt-secret\"")
	assert.Contains(t, tomlStr, "# force = true")

	// Round-trip test
	conf2 := NewJWTConfig("bla")

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
