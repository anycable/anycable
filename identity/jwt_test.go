package identity

import (
	"fmt"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTIdentifierIdentify(t *testing.T) {
	secret := "ruby-to-go"
	algo := defaultJWTAlgo
	ids := "{\"user_id\":\"15\"}"

	config := NewJWTConfig(secret)
	subject := NewJWTIdentifier(&config)

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
		assert.Equal(t, []string{"{\"type\":\"welcome\"}"}, res.Transmissions)
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
		assert.Equal(t, []string{"{\"type\":\"welcome\"}"}, res.Transmissions)
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

		enforced := NewJWTIdentifier(&config)

		env := common.NewSessionEnv("ws://demo.anycable.io/cable", nil)

		res, err := enforced.Identify("12", env)

		require.Nil(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "", res.Identifier)
		assert.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{"{\"type\":\"disconnect\",\"reason\":\"unauthorized\",\"reconnect\":false}"}, res.Transmissions)
	})
}
