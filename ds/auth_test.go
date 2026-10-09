package ds

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/streams"
	"github.com/anycable/anycable-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDSHandler_SignedStreamAuthorization(t *testing.T) {
	const secret = "s3Krit"

	ctx := context.Background()
	logger := slog.Default()

	config := node.NewConfig()
	config.HubGopoolSize = 2
	config.DisconnectMode = node.DISCONNECT_MODE_NEVER

	controller := &mocks.Controller{}
	controller.On("Shutdown").Return(nil)

	n := node.NewNode(&config, node.WithController(controller), node.WithInstrumenter(metrics.NewMetrics(nil, 10, logger)))
	n.SetDisconnector(node.NewNoopDisconnector())

	bconf := broker.NewConfig()
	brk := broker.NewMemoryBroker(pubsub.NewLegacySubscriber(n), n, &bconf)
	brk.SetEpoch("test-epoch")
	n.SetBroker(brk)
	require.NoError(t, brk.Start(nil))

	go n.Start()          // nolint:errcheck
	defer n.Shutdown(ctx) // nolint:errcheck

	require.NoError(t, brk.HandleBroadcast(&common.StreamMessage{Stream: "chat:2021", Data: `{"msg":"hello"}`}))
	require.NoError(t, brk.HandleBroadcast(&common.StreamMessage{Stream: "admin/confidential", Data: `{"classified":true}`}))

	streamsConf := streams.NewConfig()
	streamsConf.Secret = secret
	streamsConf.Public = false
	streamCtrl := streams.NewStreamsController(&streamsConf, logger)

	dsConfig := NewConfig()
	dsConfig.Path = "/ds"
	dsConfig.SkipAuth = true
	dsConfig.PollInterval = 1

	handler, err := DSHandler(n, brk, streamCtrl, nil, ctx, &server.DefaultHeadersExtractor{}, &dsConfig, logger)
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.Handle("/ds/", handler)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	signed, err := utils.NewMessageVerifier(secret).Generate("chat:2021")
	require.NoError(t, err)

	request := func(t *testing.T, method string, path string, query string, header string) (int, string) {
		t.Helper()

		req, err := http.NewRequest(method, ts.URL+path+"?"+query, nil)
		require.NoError(t, err)

		if header != "" {
			req.Header.Set(SignedStreamHeader, header)
		}

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		return resp.StatusCode, string(body)
	}

	signedQuery := "signed=" + url.QueryEscape(signed)

	for _, tc := range []struct {
		name   string
		method string
		query  string
	}{
		{"HEAD", http.MethodHead, ""},
		{"catch-up", http.MethodGet, "offset=-1"},
		{"long-poll", http.MethodGet, "offset=-1&live=long-poll"},
		{"SSE", http.MethodGet, "offset=-1&live=sse"},
	} {
		t.Run(tc.name+": rejects signed stream for a different path", func(t *testing.T) {
			query := signedQuery
			if tc.query != "" {
				query = tc.query + "&" + query
			}

			status, body := request(t, tc.method, "/ds/admin/confidential", query, "")

			assert.Equal(t, http.StatusUnauthorized, status)
			assert.NotContains(t, body, "classified")
		})

		t.Run(tc.name+": rejects signed stream header for a different path", func(t *testing.T) {
			status, body := request(t, tc.method, "/ds/admin/confidential", tc.query, signed)

			assert.Equal(t, http.StatusUnauthorized, status)
			assert.NotContains(t, body, "classified")
		})
	}

	t.Run("rejects unsigned stream when public streams are disabled", func(t *testing.T) {
		status, body := request(t, http.MethodGet, "/ds/admin/confidential", "offset=-1", "")

		assert.Equal(t, http.StatusUnauthorized, status)
		assert.NotContains(t, body, "classified")
	})

	t.Run("allows signed stream matching the path", func(t *testing.T) {
		status, body := request(t, http.MethodGet, "/ds/chat:2021", "offset=-1&"+signedQuery, "")

		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, `[{"msg":"hello"}]`, body)
	})

	t.Run("allows signed stream header matching the path", func(t *testing.T) {
		status, body := request(t, http.MethodGet, "/ds/chat:2021", "offset=-1", signed)

		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, `[{"msg":"hello"}]`, body)
	})
}
