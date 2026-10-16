package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlags_RequiresClientsAndWaves(t *testing.T) {
	var stderr bytes.Buffer
	assert.Equal(t, 2, run([]string{"-c", "10"}, &bytes.Buffer{}, &stderr))
	assert.Contains(t, stderr.String(), "--waves must be set")

	stderr.Reset()
	assert.Equal(t, 2, run([]string{"-c", "10", "--waves", "5,20"}, &bytes.Buffer{}, &stderr))
	assert.Contains(t, stderr.String(), "exceeds -c")
}

func TestParseFlags_Waves(t *testing.T) {
	cfg, err := parseFlags([]string{"-c", "100", "--waves", "10, 20,30"}, &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, []int{10, 20, 30}, cfg.waves)
	assert.Equal(t, 3, cfg.s)
	assert.Equal(t, 100*time.Millisecond, cfg.rpcLatency)
}

func TestRun_SmallWavesRecover(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"-c", "30", "-s", "2", "--waves", "10,20",
		"--wave-interval", "2s", "--settle", "100ms", "--recovery-timeout", "10s",
		"--rpc-latency", "2ms", "--backoff-min", "20ms", "--non-interactive",
	}, &stdout, &stderr)

	out := stdout.String()
	require.Equal(t, 0, code, out+stderr.String())
	assert.Contains(t, out, "setup_ok=true\n")
	assert.Contains(t, out, "recovered=true\n")
	assert.Contains(t, out, "wave_1_dropped=10\n")
	assert.Contains(t, out, "wave_2_dropped=20\n")
	assert.Contains(t, out, "close_dropped=30\n")
	// The only closes are the 30 drops; each one is followed by a reconnect.
	assert.Contains(t, out, "reconnects=30\n")
	assert.NotContains(t, out, "wave_2_recovery_seconds=-1")
	assert.NotContains(t, out, "wave_2_drained_seconds=-1")
}

func TestRun_SetupFailsWhenSubscribeAlwaysTimesOut(t *testing.T) {
	var stdout bytes.Buffer
	code := run([]string{
		"-c", "5", "--waves", "1",
		"--rpc-command-latency", "500ms", "--sub-retry-interval", "100ms", "--sub-retries-max", "0",
		"--setup-timeout", "1s", "--backoff-min", "50ms", "--non-interactive",
	}, &stdout, &bytes.Buffer{})

	out := stdout.String()
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "setup_ok=false\n")
	assert.Contains(t, out, "waves_fired=0\n")
	assert.False(t, strings.Contains(out, "close_subscribe_timeout=0\n"), out)
}

func TestCommandStall(t *testing.T) {
	mk := func(sec int, subscribing, commands int64) sample {
		s := sample{at: time.Duration(sec) * time.Second}
		s.states[1] = subscribing
		s.rpcDone[1] = commands
		return s
	}
	samples := []sample{
		mk(0, 0, 0),
		mk(1, 5, 10), // progress
		mk(2, 5, 10), // stall starts at 1s
		mk(3, 5, 10),
		mk(4, 5, 12), // progress
		mk(5, 5, 12), // stall again
	}
	cur, longest := commandStall(samples)
	assert.Equal(t, time.Second, cur)
	assert.Equal(t, 2*time.Second, longest)
}

func TestRun_InterruptPrintsSummary(t *testing.T) {
	cfg, err := parseFlags([]string{
		"-c", "10", "-s", "1", "--waves", "5",
		"--settle", "100ms", "--recovery-timeout", "1m",
		"--rpc-latency", "2ms", "--rpc-disconnect-latency", "3s",
		"--backoff-min", "20ms", "--non-interactive",
	}, &bytes.Buffer{})
	require.NoError(t, err)

	// Slow Disconnect calls keep the queue non-empty, so the run would wait
	// for the queue to drain without the interrupt.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var stdout bytes.Buffer
	began := time.Now()
	code := runWithConfig(ctx, cfg, &stdout, &bytes.Buffer{})

	out := stdout.String()
	assert.Equal(t, 130, code, out)
	assert.Less(t, time.Since(began), 15*time.Second)
	assert.Contains(t, out, "interrupted=true\n")
	// Clients recover fast; the run then waits for the slow queue to drain.
	assert.Contains(t, out, "recovered=true\n")
	assert.NotContains(t, out, "wave_1_recovery_seconds=-1")
	assert.Contains(t, out, "wave_1_drained_seconds=-1.00\n")
}
