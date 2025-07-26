package pusher

import (
	"crypto/md5" // #nosec G501
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/anycable/anycable-go/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpHandler(t *testing.T) {
	handler := &mocks.Handler{}
	config := NewConfig()
	config.AppID = "my-app-key"
	config.AppKey = "secret"
	config.Secret = "test-push"

	verifier := NewVerifier("my-app-key", "test-push")

	broadcaster := NewBroadcaster(handler, &config, slog.Default())

	payload := []byte(`{"name":"test-event","channel":"test-channel","data":"{\"message\":\"hello world\"}"}`)
	payloadMD5 := md5.Sum(payload) // #nosec G401

	broadcastPayload := []byte(`{"stream":"test-channel","data":"{\"event\":\"test-event\",\"data\":\"{\\\"message\\\":\\\"hello world\\\"}\"}"}`)

	handler.On(
		"HandleBroadcast",
		broadcastPayload,
	)

	t.Run("Handles broadcasts", func(t *testing.T) {
		authParams := "auth_key=" + config.AppKey +
			"&auth_timestamp=" + strconv.FormatInt(time.Now().Unix(), 10) +
			"&auth_version=1.0" +
			"&body_md5=" + fmt.Sprintf("%x", payloadMD5)

		stringToSign := "POST\n/events\n" + authParams
		authSignature := verifier.Sign(stringToSign)

		req, err := http.NewRequest("POST", "/events?"+authParams+"&auth_signature="+authSignature, strings.NewReader(string(payload)))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(broadcaster.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Rejects non-POST requests", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/", strings.NewReader(string(payload)))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(broadcaster.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	})

	t.Run("Rejects when authorization data is incorrect", func(t *testing.T) {
		authParams := "auth_key=" + config.AppKey +
			"&body_md5=" + fmt.Sprintf("%x", payloadMD5) +
			"&auth_version=1.0" +
			"&auth_timestamp=" + strconv.FormatInt(time.Now().Unix(), 10)

		req, err := http.NewRequest("POST", "/events?"+authParams+"&auth_signature=qwewe123wqee1eqweqweqwe", strings.NewReader(string(payload)))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(broadcaster.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}
