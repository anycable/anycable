package lib_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anycable/anycable-go/etc/benchi/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildServer_Defaults(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	require.NotNil(t, srv)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	bURL := srv.BroadcastURL()
	wsURL := srv.WebSocketURL()

	require.NotEmpty(t, bURL)
	require.NotEmpty(t, wsURL)
	assert.Contains(t, bURL, "127.0.0.1")
	assert.Contains(t, wsURL, "127.0.0.1")
	assert.True(t, strings.HasPrefix(wsURL, "ws://"), "websocket URL must use ws:// scheme, got %q", wsURL)
	assert.True(t, strings.HasSuffix(bURL, "/broadcast"), "broadcast URL must end in /broadcast, got %q", bURL)
	assert.True(t, strings.HasSuffix(wsURL, "/cable"), "websocket URL must end in /cable, got %q", wsURL)

	require.NoError(t, srv.Shutdown(context.Background()))
}

func TestBuildServer_BroadcastURL_AcceptsPOST(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	body := strings.NewReader(`{"stream":"foo","data":"x"}`)
	req, err := http.NewRequest(http.MethodPost, srv.BroadcastURL(), body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestBuildServer_LogSuppression(t *testing.T) {
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	require.NoError(t, srv.Shutdown(context.Background()))

	require.NoError(t, w.Close())
	os.Stderr = origStderr

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	assert.Empty(t, buf.String(), "no server output expected on stderr, got: %s", buf.String())
}

func TestBuildServer_BrokerSwap(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{BrokerAdapter: "memory"})
	require.NoError(t, err)
	require.NotNil(t, srv)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
}

func TestBuildServer_DoubleShutdownReturnsNil(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)

	require.NoError(t, srv.Shutdown(context.Background()))
	assert.NotPanics(t, func() {
		err := srv.Shutdown(context.Background())
		assert.NoError(t, err)
	})
}

func TestBuildServer_StartsAndStops_InOneSecond(t *testing.T) {
	start := time.Now()
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	require.NoError(t, srv.Shutdown(context.Background()))
	elapsed := time.Since(start)

	assert.Less(t, elapsed, time.Second, "BuildServer + Shutdown took %s, expected <1s", elapsed)
}
