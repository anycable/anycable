package pusher

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

type Verifier struct {
	app_id string
	key    string
}

func NewVerifier(app_id string, key string) *Verifier {
	return &Verifier{app_id, key}
}

func (v *Verifier) VerifyChannel(socket string, channel string, signature string) bool {
	string_to_sign := socket + ":" + channel

	return v.verifyString(string_to_sign, signature)
}

func (v *Verifier) VerifyPresenceChannel(socket string, channel string, info string, signature string) bool {
	string_to_sign := socket + ":" + channel + ":" + info

	return v.verifyString(string_to_sign, signature)
}

func (v *Verifier) VerifyUser(socket string, user_info string, signature string) bool {
	string_to_sign := socket + "::user::" + user_info

	return v.verifyString(string_to_sign, signature)
}

func (v *Verifier) verifyString(input string, signature string) bool {
	h := hmac.New(sha256.New, []byte(v.key))
	h.Write([]byte(input))
	calculated_signature := hex.EncodeToString(h.Sum(nil))

	calculated_auth := v.app_id + ":" + calculated_signature

	return hmac.Equal([]byte(calculated_auth), []byte(signature))
}
