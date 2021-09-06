package identity

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/anycable/anycable-go/common"
	"github.com/apex/log"
	"github.com/golang-jwt/jwt"
)

const (
	expiredMessage = "{\"type\":\"disconnect\",\"reason\":\"token_expired\",\"reconnect\":false}"
)

type JWTConfig struct {
	Secret string
	Param  string
	Algo   jwt.SigningMethod
	Force  bool
}

var (
	defaultJWTAlgo = jwt.SigningMethodHS256
)

func NewJWTConfig(secret string) JWTConfig {
	return JWTConfig{Secret: secret, Param: "jid", Algo: defaultJWTAlgo}
}

func (c JWTConfig) Enabled() bool {
	return c.Secret != ""
}

type JWTIdentifier struct {
	secret     []byte
	paramName  string
	headerName string
	required   bool
	log        *log.Entry
}

var _ Identifier = (*JWTIdentifier)(nil)

func NewJWTIdentifier(config *JWTConfig) *JWTIdentifier {
	return &JWTIdentifier{
		secret:     []byte(config.Secret),
		paramName:  config.Param,
		headerName: strings.ToLower(fmt.Sprintf("x-%s", config.Param)),
		required:   config.Force,
		log:        log.WithField("context", "jwt"),
	}
}

func (i *JWTIdentifier) Identify(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	var rawToken string

	if env.Headers != nil {
		if v, ok := (*env.Headers)[i.headerName]; ok {
			rawToken = v
		}
	}

	if rawToken == "" {
		u, err := url.Parse(env.URL)

		if err != nil {
			return nil, err
		}

		m, err := url.ParseQuery(u.RawQuery)

		if err != nil {
			return nil, err
		}

		if v, ok := m[i.paramName]; ok {
			rawToken = v[0]
		}
	}

	if rawToken == "" {
		i.log.Debugf("No token is found (url=%s, headers=%v)", env.URL, env.Headers)

		if i.required {
			return unauthorizedResponse(), nil
		}

		return nil, nil
	}

	token, err := jwt.Parse(rawToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return i.secret, nil
	})

	if err != nil {
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&(jwt.ValidationErrorExpired) != 0 {
				i.log.Debugf("Token has expired")

				return expiredResponse(), nil
			}
		}

		i.log.Debugf("Invalid token: %v", err)
		return unauthorizedResponse(), nil
	}

	var ids string

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if v, ok := claims["ext"].(string); ok {
			ids = v
		} else {
			return nil, fmt.Errorf("JWT token doesn't contain identifiers: %v", claims)
		}
	} else {
		return nil, err
	}

	return &common.ConnectResult{
		Identifier:    ids,
		Transmissions: []string{actionCableWelcomeMessage},
		Status:        common.SUCCESS,
	}, nil
}

func unauthorizedResponse() *common.ConnectResult {
	return &common.ConnectResult{Status: common.FAILURE, Transmissions: []string{actionCableDisconnectUnauthorizedMessage}}
}

func expiredResponse() *common.ConnectResult {
	return &common.ConnectResult{Status: common.FAILURE, Transmissions: []string{expiredMessage}}
}
