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
)

type MessageVerifier struct {
	key []byte
}

func NewMessageVerifier(key string) *MessageVerifier {
	return &MessageVerifier{key: []byte(key)}
}

func (m *MessageVerifier) Verified(msg string) (string, error) {
	if !m.isValid(msg) {
		return "", errors.New("Invalid message")
	}

	parts := strings.Split(msg, "--")
	data := parts[0]

	jsonStr, err := base64.StdEncoding.DecodeString(data)

	if err != nil {
		return "", err
	}

	var result string

	if err = json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return "", err
	}

	return result, nil
}

// https://github.com/rails/rails/blob/061bf3156fb90ac6b8ec255dfa39492cf22d7b13/activesupport/lib/active_support/message_verifier.rb#L122
func (m *MessageVerifier) isValid(msg string) bool {
	if msg == "" {
		return false
	}

	parts := strings.Split(msg, "--")

	if len(parts) != 2 {
		return false
	}

	data := []byte(parts[0])
	digest := []byte(parts[1])

	h := hmac.New(sha256.New, []byte(m.key))
	h.Write(data)

	actual := []byte(fmt.Sprintf("%x", h.Sum(nil)))

	return subtle.ConstantTimeEq(int32(len(actual)), int32(len(digest))) == 1 &&
		subtle.ConstantTimeCompare(actual[:], digest) == 1
}
