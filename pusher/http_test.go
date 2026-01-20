package pusher

import (
	"context"
	"crypto/md5" // #nosec G501
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/mocks"
	"github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpHandler(t *testing.T) {
	handler := &mocks.Handler{}
	brokerMock := &mocks.Broker{}
	config := NewConfig()
	config.AppID = "my-app-id"
	config.AppKey = "my-app-key"
	config.Secret = "test-push"

	verifier := NewVerifier(config.AppKey, config.Secret)

	restAPI := NewRestAPI(handler, brokerMock, &config, slog.Default())

	payload := []byte(`{"name":"test-event","channel":"test-channel","data":"{\"message\":\"hello world\"}"}`)
	payloadMD5 := md5.Sum(payload) // #nosec G401

	broadcastPayload := []byte(`{"stream":"test-channel","data":"{\"event\":\"test-event\",\"data\":\"{\\\"message\\\":\\\"hello world\\\"}\"}"}`)

	handler.On(
		"HandleBroadcast",
		broadcastPayload,
	).Return(nil)

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
		handler := http.HandlerFunc(restAPI.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Rejects unsupported methods", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", "/", strings.NewReader(string(payload)))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(restAPI.Handler)
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
		handler := http.HandlerFunc(restAPI.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestGetUsersIntegration(t *testing.T) {
	config := NewConfig()
	config.AppID = "app-id"
	config.AppKey = "app-key"
	config.Secret = "secret"

	bconf := broker.NewConfig()
	memBroker := broker.NewMemoryBroker(nil, nil, &bconf)
	require.NoError(t, memBroker.Start(nil))
	defer memBroker.Shutdown(context.Background()) // nolint:errcheck

	handler := &mocks.Handler{}
	restAPI := NewRestAPI(handler, memBroker, &config, slog.Default())

	server := httptest.NewServer(http.HandlerFunc(restAPI.Handler))
	defer server.Close()

	client := pusher.Client{
		AppID:   config.AppID,
		Key:     config.AppKey,
		Secret:  config.Secret,
		Cluster: "local",
		Host:    strings.TrimPrefix(server.URL, "http://"),
	}

	t.Run("Get users from presence channel with users", func(t *testing.T) {
		channel := "presence-room-1"

		_, err := memBroker.PresenceAdd(channel, "session-1", "user-1", map[string]string{"name": "Alice"})
		require.NoError(t, err)
		_, err = memBroker.PresenceAdd(channel, "session-2", "user-2", map[string]string{"name": "Bob"})
		require.NoError(t, err)

		users, err := client.GetChannelUsers(channel)
		require.NoError(t, err)

		assert.Len(t, users.List, 2)

		ids := []string{users.List[0].ID, users.List[1].ID}
		assert.Contains(t, ids, "user-1")
		assert.Contains(t, ids, "user-2")
	})

	t.Run("Get users from empty channel", func(t *testing.T) {
		channel := "presence-empty-room"

		users, err := client.GetChannelUsers(channel)
		require.NoError(t, err)

		assert.Len(t, users.List, 0)
	})

	t.Run("Get users from unknown channel", func(t *testing.T) {
		channel := "presence-unknown"

		users, err := client.GetChannelUsers(channel)
		require.NoError(t, err)

		assert.Len(t, users.List, 0)
	})

	t.Run("Get users from non-presence channel returns 400", func(t *testing.T) {
		channel := "private-room"

		_, err := client.GetChannelUsers(channel)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "400")
	})
}
