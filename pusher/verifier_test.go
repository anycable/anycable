package pusher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Based on the official docs: https://pusher.com/docs/channels/library_auth_reference/auth-signatures/#worked-examples
func TestVerifyChannel(t *testing.T) {
	v := NewVerifier("278d425bdf160c739803", "7ad3773142a6692b25b8")

	assert.True(t, v.VerifyChannel("1234.1234", "private-foobar", "278d425bdf160c739803:58df8b0c36d6982b82c3ecf6b4662e34fe8c25bba48f5369f135bf843651c3a4"))
	// channel mismatch
	assert.False(t, v.VerifyChannel("1234.1234", "foobar", "278d425bdf160c739803:58df8b0c36d6982b82c3ecf6b4662e34fe8c25bba48f5369f135bf843651c3a4"))
	// signature mismatch
	assert.False(t, v.VerifyChannel("1234.1234", "private-foobar", "278d425bdf160c739803:df8b0c36d6982b82c3ecf6b4662e34fe8c25bba48f5369f135bf843651c3a458"))
	// socket mismatch
	assert.False(t, v.VerifyChannel("1234.4321", "private-foobar", "278d425bdf160c739803:58df8b0c36d6982b82c3ecf6b4662e34fe8c25bba48f5369f135bf843651c3a4"))
}

func TestVerifyPresenceChannel(t *testing.T) {
	v := NewVerifier("278d425bdf160c739803", "7ad3773142a6692b25b8")

	assert.True(t, v.VerifyPresenceChannel("1234.1234", "presence-foobar", "{\"user_id\":10,\"user_info\":{\"name\":\"Mr. Channels\"}}", "278d425bdf160c739803:31935e7d86dba64c2a90aed31fdc61869f9b22ba9d8863bba239c03ca481bc80"))
	// signature mismatch
	assert.False(t, v.VerifyPresenceChannel("1234.1234", "presence-foobar", "{\"user_id\":10,\"user_info\":{\"name\":\"Mr. Channels\"}}", "278d425bdf160c739803:aed3695da2ffd16931f457e338e6c9f2921fa133ce7dac49f529792be6304cdd"))
	// channel mismatch
	assert.False(t, v.VerifyPresenceChannel("1234.1234", "presence-foobaz", "{\"user_id\":10,\"user_info\":{\"name\":\"Mr. Channels\"}}", "278d425bdf160c739803:31935e7d86dba64c2a90aed31fdc61869f9b22ba9d8863bba239c03ca481bc80"))
	// socket mismatch
	assert.False(t, v.VerifyPresenceChannel("1234.4321", "presence-foobar", "{\"user_id\":10,\"user_info\":{\"name\":\"Mr. Channels\"}}", "278d425bdf160c739803:31935e7d86dba64c2a90aed31fdc61869f9b22ba9d8863bba239c03ca481bc80"))
	// data mismatch
	assert.False(t, v.VerifyPresenceChannel("1234.1234", "presence-foobar", "{\"id\":10,\"user_info\":{\"name\":\"Mr. Channels\"}}", "278d425bdf160c739803:31935e7d86dba64c2a90aed31fdc61869f9b22ba9d8863bba239c03ca481bc80"))
}

func TestVerifyUser(t *testing.T) {
	v := NewVerifier("278d425bdf160c739803", "7ad3773142a6692b25b8")

	assert.True(t, v.VerifyUser("1234.1234", "{\"id\":\"12345\"}", "278d425bdf160c739803:4708d583dada6a56435fb8bc611c77c359a31eebde13337c16ab43aa6de336ba"))
	assert.False(t, v.VerifyUser("1234.1234", "{\"id\":\"12345\"}", "278d425bdf160c739803:df8b0c36d6982b82c3ecf6b4662e34fe8c25bba48f5369f135bf843651c3a458"))
	assert.False(t, v.VerifyUser("1234.4321", "{\"id\":\"12345\"}", "278d425bdf160c739803:4708d583dada6a56435fb8bc611c77c359a31eebde13337c16ab43aa6de336ba"))
	assert.False(t, v.VerifyUser("1234.1234", "{\"user_id\":\"12345\"}", "278d425bdf160c739803:4708d583dada6a56435fb8bc611c77c359a31eebde13337c16ab43aa6de336ba"))
}
