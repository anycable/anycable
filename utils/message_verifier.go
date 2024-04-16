package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/joomcode/errorx"
)

type MessageVerifier struct {
	key []byte
}

func NewMessageVerifier(key string) *MessageVerifier {
	return &MessageVerifier{key: []byte(key)}
}

func (m *MessageVerifier) Generate(payload interface{}) (string, error) {
	payloadJson, err := json.Marshal(payload)

	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(payloadJson)

	signature, err := m.Sign([]byte(encoded))

	if err != nil {
		return "", err
	}

	signed := encoded + "--" + string(signature)
	return signed, nil
}

func (m *MessageVerifier) Verified(msg string) (interface{}, error) {
	if err := m.Validate(msg); err != nil {
		return "", errorx.Decorate(err, "failed to verify message")
	}

	parts := strings.Split(msg, "--")
	data := parts[0]

	jsonStr, err := base64.StdEncoding.DecodeString(data)

	if err != nil {
		return "", err
	}

	var result interface{}

	if err = json.Unmarshal(jsonStr, &result); err != nil {
		return "", err
	}

	return result, nil
}

// https://github.com/rails/rails/blob/061bf3156fb90ac6b8ec255dfa39492cf22d7b13/activesupport/lib/active_support/message_verifier.rb#L122
func (m *MessageVerifier) Validate(msg string) error {
	if msg == "" {
		return errors.New("message is empty")
	}

	parts := strings.Split(msg, "--")

	if len(parts) != 2 {
		return fmt.Errorf("message must contain 2 parts, got %d", len(parts))
	}

	data := []byte(parts[0])
	digest := []byte(parts[1])

	if m.VerifySignature(data, digest) {
		return nil
	} else {
		return errors.New("invalid signature")
	}
}

func (m *MessageVerifier) Sign(payload []byte) ([]byte, error) {
	digest := hmac.New(sha256.New, m.key)
	_, err := digest.Write(payload)

	if err != nil {
		return nil, errorx.Decorate(err, "failed to sign payload")
	}

	return []byte(fmt.Sprintf("%x", digest.Sum(nil))), nil
}

func (m *MessageVerifier) VerifySignature(payload []byte, digest []byte) bool {
	h := hmac.New(sha256.New, m.key)
	h.Write(payload)

	actual := []byte(fmt.Sprintf("%x", h.Sum(nil)))

	return subtle.ConstantTimeEq(int32(len(actual)), int32(len(digest))) == 1 &&
		subtle.ConstantTimeCompare(actual, digest) == 1
}
