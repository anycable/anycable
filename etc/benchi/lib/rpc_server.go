package lib

import (
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"time"

	pb "github.com/anycable/anycable-go/protos"
)

// RPCMethod identifies one of the three AnyCable RPC methods.
type RPCMethod int

const (
	RPCConnect RPCMethod = iota
	RPCCommand
	RPCDisconnect
	rpcMethodsNum
)

func (m RPCMethod) String() string {
	switch m {
	case RPCConnect:
		return "connect"
	case RPCCommand:
		return "command"
	case RPCDisconnect:
		return "disconnect"
	}
	return "unknown"
}

// RPCServerConfig configures the fake HTTP RPC server.
//
// Latency is the default per-call delay for all methods. A per-method
// latency of zero falls back to Latency. Jitter is a fraction in [0, 1]:
// each call sleeps for latency × (1 ± Jitter), uniformly distributed.
type RPCServerConfig struct {
	Latency           time.Duration
	ConnectLatency    time.Duration
	CommandLatency    time.Duration
	DisconnectLatency time.Duration
	Jitter            float64
}

func (c RPCServerConfig) latencyFor(m RPCMethod) time.Duration {
	var l time.Duration
	switch m {
	case RPCConnect:
		l = c.ConnectLatency
	case RPCCommand:
		l = c.CommandLatency
	case RPCDisconnect:
		l = c.DisconnectLatency
	}
	if l == 0 {
		l = c.Latency
	}
	return l
}

// RPCMethodStats is a point-in-time view of one method's counters.
type RPCMethodStats struct {
	Started   int64
	Completed int64
	InFlight  int64
}

type rpcCounters struct {
	started   atomic.Int64
	completed atomic.Int64
}

// RPCServer is an in-process fake of an AnyCable HTTP RPC application. It
// accepts every connection, confirms every subscription (streaming from
// "bench:<identifier>"), and acknowledges every disconnect — after a
// configurable, jittered delay. It counts started/completed calls per method
// so scenarios can observe which calls reach the RPC side.
type RPCServer struct {
	cfg      RPCServerConfig
	ts       *httptest.Server
	counters [rpcMethodsNum]rpcCounters
	nextID   atomic.Int64
}

// NewRPCServer starts the fake RPC server on a loopback address.
func NewRPCServer(cfg RPCServerConfig) *RPCServer {
	s := &RPCServer{cfg: cfg}
	mux := http.NewServeMux()
	mux.HandleFunc("/connect", s.handle(RPCConnect, s.connect))
	mux.HandleFunc("/command", s.handle(RPCCommand, s.command))
	mux.HandleFunc("/disconnect", s.handle(RPCDisconnect, s.disconnect))
	s.ts = httptest.NewServer(mux)
	return s
}

// URL returns the base URL to pass as the RPC host.
func (s *RPCServer) URL() string { return s.ts.URL }

// Close stops the server. In-flight calls are interrupted.
func (s *RPCServer) Close() {
	s.ts.CloseClientConnections()
	s.ts.Close()
}

// Stats returns the counters for the given method.
func (s *RPCServer) Stats(m RPCMethod) RPCMethodStats {
	c := &s.counters[m]
	completed := c.completed.Load()
	started := c.started.Load()
	return RPCMethodStats{Started: started, Completed: completed, InFlight: started - completed}
}

func (s *RPCServer) handle(m RPCMethod, fn func(body []byte) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := &s.counters[m]
		c.started.Add(1)
		defer c.completed.Add(1)

		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}

		s.sleep(m, r)

		resp, err := fn(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func (s *RPCServer) sleep(m RPCMethod, r *http.Request) {
	d := s.cfg.latencyFor(m)
	if d <= 0 {
		return
	}
	if j := s.cfg.Jitter; j > 0 {
		d = time.Duration(float64(d) * (1 + j*(2*rand.Float64()-1))) //nolint:gosec // jitter, not crypto
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-r.Context().Done():
	}
}

func (s *RPCServer) connect(_ []byte) (any, error) {
	id := s.nextID.Add(1)
	return &pb.ConnectionResponse{
		Status:        pb.Status_SUCCESS,
		Identifiers:   `{"id":` + jsonString(strconv.FormatInt(id, 10)) + `}`,
		Transmissions: []string{`{"type":"welcome"}`},
	}, nil
}

func (s *RPCServer) command(body []byte) (any, error) {
	var req pb.CommandMessage
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	switch req.Command {
	case "subscribe":
		confirm, _ := json.Marshal(map[string]string{
			"identifier": req.Identifier,
			"type":       "confirm_subscription",
		})
		return &pb.CommandResponse{
			Status:        pb.Status_SUCCESS,
			Streams:       []string{"bench:" + req.Identifier},
			Transmissions: []string{string(confirm)},
		}, nil
	case "unsubscribe":
		return &pb.CommandResponse{Status: pb.Status_SUCCESS, StopStreams: true}, nil
	default:
		return &pb.CommandResponse{Status: pb.Status_SUCCESS}, nil
	}
}

func (s *RPCServer) disconnect(_ []byte) (any, error) {
	return &pb.DisconnectResponse{Status: pb.Status_SUCCESS}, nil
}

func jsonString(v string) string {
	b, _ := json.Marshal(v)
	return string(b)
}
