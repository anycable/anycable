package lib

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRPCServer_AfterClose(t *testing.T) {
	rpc := NewRPCServer(RPCServerConfig{SessionClosed: func(sid string) bool { return sid == "closed" }})
	t.Cleanup(rpc.Close)

	post := func(path, body string) {
		t.Helper()
		resp, err := http.Post(rpc.URL()+path, "application/json", strings.NewReader(body)) //nolint:noctx // test
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	post("/connect", `{"env":{"url":"ws://localhost/cable?bench_sid=closed"}}`)
	post("/connect", `{"env":{"url":"ws://localhost/cable?bench_sid=open"}}`)
	post("/command", `{"command":"subscribe","identifier":"x","connection_identifiers":"{\"id\":\"closed\"}"}`)
	post("/command", `{"command":"subscribe","identifier":"x","connection_identifiers":"{\"id\":\"open\"}"}`)
	post("/disconnect", `{"identifiers":"{\"id\":\"closed\"}"}`)

	assert.Equal(t, int64(1), rpc.Stats(RPCConnect).AfterClose)
	assert.Equal(t, int64(1), rpc.Stats(RPCCommand).AfterClose)
	// Disconnect calls always come after the close; they are not counted.
	assert.Equal(t, int64(0), rpc.Stats(RPCDisconnect).AfterClose)
}
