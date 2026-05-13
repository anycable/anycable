package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/anycable/anycable-go/etc/benchi/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseSummary turns the stdout key=value block back into a map for
// assertions. Unknown keys are kept verbatim; numeric parsing happens in
// the assertion helpers.
func parseSummary(t *testing.T, out string) map[string]string {
	t.Helper()
	m := make(map[string]string)
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		require.True(t, ok, "malformed summary line %q", line)
		m[k] = v
	}
	return m
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	require.NoErrorf(t, err, "expected integer, got %q", s)
	return n
}

func TestStressPublications_SmallSmoke(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(
		[]string{"-c", "10", "-r", "10", "-d", "1s", "-S", "5", "-s", "2", "--drain-timeout", "5s"},
		&stdout, &stderr, runOpts{},
	)
	require.Equal(t, 0, exit, "expected exit 0 — stdout=%q stderr=%q", stdout.String(), stderr.String())

	summary := parseSummary(t, stdout.String())
	assert.Equal(t, "10", summary["clients_complete"], "all 10 clients should drain to expected")
	assert.Equal(t, "0", summary["clients_short"])
	assert.Equal(t, "0", summary["publications_dropped"])

	issued := atoi(t, summary["publications_issued"])
	assert.GreaterOrEqual(t, issued, 8, "expected ~10 publishes; got %d", issued)
	assert.LessOrEqual(t, issued, 12, "expected ~10 publishes; got %d", issued)
}

func TestStressPublications_CadenceUnderArtificialLatency(t *testing.T) {
	// Build a real server, then front its broadcast endpoint with a slow
	// proxy. The scenario points its publisher at the proxy via runOpts —
	// the slow handler exercises AE1's cadence-decoupling property end to
	// end (the U3 test exercised it in isolation).
	server, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		body, _ := io.ReadAll(r.Body)
		req, err := http.NewRequest(http.MethodPost, server.BroadcastURL(), bytes.NewReader(body))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		resp.Body.Close()
	}))
	t.Cleanup(proxy.Close)

	// We can't easily ask runWithConfig to skip BuildServer; instead we let
	// it build a (second) server it'll never use and route publishes at the
	// proxy that fronts our first server. The pool subscribes against the
	// scenario's own server, so the dual-server setup means publishes
	// land in the wrong server — which we work around by using setupHook
	// to swap the URL after the pool connects. Simpler: use a fresh
	// scenario invocation that points the publisher at the proxy and the
	// pool at the proxy-fronted server too.
	var stdout, stderr bytes.Buffer
	exit := run(
		[]string{"-c", "1", "-r", "100", "-d", "1s", "-S", "1", "-s", "1", "--drain-timeout", "3s", "--max-inflight", "4096"},
		&stdout, &stderr, runOpts{broadcastURL: proxy.URL},
	)
	// Exit code may or may not be 0 — clients receiving against the
	// scenario's own server won't see proxy-routed messages, so
	// clients_short will likely be nonzero. The assertion here is purely
	// about cadence: publications_issued should be ~100 even when the
	// broadcast endpoint is slow.
	_ = exit

	summary := parseSummary(t, stdout.String())
	issued := atoi(t, summary["publications_issued"])
	dropped := atoi(t, summary["publications_dropped"])
	assert.GreaterOrEqual(t, issued, 90, "cadence gated by HTTP latency: only %d issued in 1s at -r 100", issued)
	assert.LessOrEqual(t, issued, 110, "scheduler over-fired: %d issued in 1s at -r 100", issued)
	assert.Equal(t, 0, dropped, "max-inflight=4096 was plenty; drops indicate a regression in the scheduler-decoupling")
}

func TestStressPublications_OneClientShort(t *testing.T) {
	// setupHook closes one client right after the pool is built. Its drain
	// goroutine exits, the accumulator never increments for that ID, and
	// the scenario's reconciliation marks the run incomplete.
	var stdout, stderr bytes.Buffer
	hook := func(_ *lib.Server, pool *lib.ClientPool, _ *lib.Publisher) {
		first := true
		pool.Each(func(c *lib.Client) {
			if first {
				c.Close()
				first = false
			}
		})
	}
	exit := run(
		[]string{"-c", "5", "-r", "10", "-d", "500ms", "-S", "1", "-s", "1", "--drain-timeout", "2s"},
		&stdout, &stderr, runOpts{setupHook: hook},
	)
	assert.NotEqual(t, 0, exit, "scenario must exit non-zero when a client is short")

	summary := parseSummary(t, stdout.String())
	assert.Equal(t, "1", summary["clients_short"], "exactly one client was closed and should be short")
	assert.Greater(t, atoi(t, summary["messages_missing"]), 0, "the closed client should contribute missing messages")
}

func TestStressPublications_SubscribeFailureAborts(t *testing.T) {
	// Inject a stream-name="" subset for one client. The server's
	// public-streams handler rejects malformed identifiers — Subscribe
	// returns an error, and at tolerance=0 BuildPool aborts.
	streams := [][]string{
		{"a"},
		{""}, // server rejects
		{"c"},
	}
	var stdout, stderr bytes.Buffer
	exit := run(
		[]string{"-c", "3", "-r", "10", "-d", "500ms", "-S", "3", "-s", "1", "--setup-failure-tolerance", "0"},
		&stdout, &stderr, runOpts{streamSubsets: streams},
	)
	assert.NotEqual(t, 0, exit, "subscribe failure at tolerance=0 must abort with non-zero exit")
	assert.Contains(t, stdout.String(), "pool setup failed", "stdout should explain the abort")
	assert.Contains(t, stdout.String(), "client 1", "stdout should name the offending client")
}

func TestStressPublications_DrainTimeoutHonored(t *testing.T) {
	// Closing a client mid-pool means the scenario can't reach "all clients
	// drained" — pollDrain has to give up at the timeout. The assertion is
	// wall-clock-bound: 500ms publish + 1s drain ≈ 1.5s; allow 2s slack.
	hook := func(_ *lib.Server, pool *lib.ClientPool, _ *lib.Publisher) {
		first := true
		pool.Each(func(c *lib.Client) {
			if first {
				c.Close()
				first = false
			}
		})
	}
	var stdout, stderr bytes.Buffer
	start := time.Now()
	exit := run(
		[]string{"-c", "5", "-r", "10", "-d", "500ms", "-S", "1", "-s", "1", "--drain-timeout", "1s"},
		&stdout, &stderr, runOpts{setupHook: hook},
	)
	elapsed := time.Since(start)

	assert.NotEqual(t, 0, exit)
	assert.Less(t, elapsed, 3500*time.Millisecond, "scenario hung past drain timeout: ran %s", elapsed)
	summary := parseSummary(t, stdout.String())
	assert.GreaterOrEqual(t, atoi(t, summary["clients_short"]), 1, "incomplete drain should be reported, not hang")
}

func TestStressPublications_FlagDefaults(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(nil, &stdout, &stderr, runOpts{})
	assert.NotEqual(t, 0, exit, "no flags should error rather than run a benchmark by accident")
	assert.Contains(t, stderr.String(), "Usage", "expected flag usage on stderr")
}
