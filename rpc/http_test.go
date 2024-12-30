package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/protocol"
	pb "github.com/anycable/anycable-go/protos"
	"github.com/anycable/anycable-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestHTTPServiceRPC(t *testing.T) {
	var onRequest func(r *http.Request, w http.ResponseWriter)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if onRequest == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		onRequest(r, w)
	}))

	defer ts.Close()

	conf := NewConfig()
	conf.Host = ts.URL

	service, _ := NewHTTPService(&conf)

	t.Run("Connect", func(t *testing.T) {
		onRequest = func(r *http.Request, w http.ResponseWriter) {
			require.Equal(t, "/connect", r.URL.Path)

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var req pb.ConnectionRequest
			err = json.Unmarshal(body, &req)
			require.NoError(t, err)

			require.Equal(t, "ws://anycable.io/cable", req.Env.Url)
			require.Equal(t, "foo=bar", req.Env.Headers["cookie"])

			identifiers := fmt.Sprintf("%s-%s", r.Header.Get("x-anycable-meta-year"), r.Header.Get("x-anycable-meta-album"))

			res := pb.ConnectionResponse{
				Transmissions: []string{"welcome"},
				Identifiers:   identifiers,
				Status:        pb.Status_SUCCESS,
			}

			w.Write(utils.ToJSON(&res)) // nolint: errcheck
		}

		md := metadata.Pairs("album", "Kamni", "year", "2008")
		ctx := metadata.NewIncomingContext(context.Background(), md)
		res, err := service.Connect(ctx, protocol.NewConnectMessage(
			common.NewSessionEnv("ws://anycable.io/cable", &map[string]string{"cookie": "foo=bar"}),
		))

		require.NoError(t, err)

		assert.Equal(t, pb.Status_SUCCESS, res.Status)
		assert.Equal(t, []string{"welcome"}, res.Transmissions)
		assert.Equal(t, "2008-Kamni", res.Identifiers)
	})

	t.Run("Disconnect", func(t *testing.T) {
		onRequest = func(r *http.Request, w http.ResponseWriter) {
			require.Equal(t, "/disconnect", r.URL.Path)

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var req pb.DisconnectRequest
			err = json.Unmarshal(body, &req)
			require.NoError(t, err)

			require.Equal(t, "ws://anycable.io/cable", req.Env.Url)
			require.Equal(t, "foo=bar", req.Env.Headers["cookie"])
			require.Equal(t, "test-session", req.Identifiers)

			res := pb.DisconnectResponse{
				Status:   pb.Status_ERROR,
				ErrorMsg: r.Header.Get("x-anycable-meta-error"),
			}

			w.Write(utils.ToJSON(&res)) // nolint: errcheck
		}

		md := metadata.Pairs("error", "test error")
		ctx := metadata.NewIncomingContext(context.Background(), md)
		res, err := service.Disconnect(ctx, protocol.NewDisconnectMessage(
			common.NewSessionEnv("ws://anycable.io/cable", &map[string]string{"cookie": "foo=bar"}),
			"test-session",
			[]string{},
		))

		require.NoError(t, err)

		assert.Equal(t, pb.Status_ERROR, res.Status)
		assert.Equal(t, "test error", res.ErrorMsg)
	})

	t.Run("Command", func(t *testing.T) {
		onRequest = func(r *http.Request, w http.ResponseWriter) {
			require.Equal(t, "/command", r.URL.Path)

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var req pb.CommandMessage
			err = json.Unmarshal(body, &req)
			require.NoError(t, err)

			require.Equal(t, "chat_1", req.Identifier)
			require.Equal(t, "subscribe", req.Command)

			stream := r.Header.Get("x-anycable-meta-track")

			res := pb.CommandResponse{
				Transmissions: []string{"confirmed"},
				Streams:       []string{stream},
				Status:        pb.Status_SUCCESS,
				Presence: &pb.PresenceResponse{
					Type: "join",
					Id:   "42",
					Info: `{"name":"Dexter"}`,
				},
			}

			w.Write(utils.ToJSON(&res)) // nolint: errcheck
		}

		md := metadata.Pairs("track", "easy-way-out")
		ctx := metadata.NewIncomingContext(context.Background(), md)
		res, err := service.Command(ctx, protocol.NewCommandMessage(
			common.NewSessionEnv("ws://anycable.io/cable", &map[string]string{"cookie": "foo=bar"}),
			"subscribe",
			"chat_1",
			"test-session",
			"{}",
		))

		require.NoError(t, err)

		assert.Equal(t, pb.Status_SUCCESS, res.Status)
		assert.Equal(t, []string{"confirmed"}, res.Transmissions)
		assert.Equal(t, []string{"easy-way-out"}, res.Streams)
		assert.Equal(t, "join", res.Presence.Type)
		assert.Equal(t, "42", res.Presence.Id)
		assert.Equal(t, `{"name":"Dexter"}`, res.Presence.Info)
	})
}

func TestHTTPServiceAuthentication(t *testing.T) {
	conf := NewConfig()
	conf.Secret = "secretto"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secretto" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		res := pb.ConnectionResponse{
			Status: pb.Status_SUCCESS,
		}

		w.Write(utils.ToJSON(&res)) // nolint: errcheck
	}))

	defer ts.Close()

	conf.Host = ts.URL

	service, _ := NewHTTPService(&conf)

	request := protocol.NewConnectMessage(
		common.NewSessionEnv("ws://anycable.io/cable", &map[string]string{"cookie": "foo=bar"}),
	)

	t.Run("Authentication_SUCCESS", func(t *testing.T) {
		res, err := service.Connect(context.Background(), request)

		require.NoError(t, err)

		assert.Equal(t, pb.Status_SUCCESS, res.Status)
	})

	t.Run("Authentication_FAILURE", func(t *testing.T) {
		newConf := NewConfig()
		newConf.Secret = "not-a-secret"
		newConf.Host = ts.URL

		service, _ := NewHTTPService(&newConf)

		_, err := service.Connect(context.Background(), request)

		require.Error(t, err)

		grpcErr, ok := status.FromError(err)

		require.True(t, ok)

		assert.Equal(t, codes.Unauthenticated, grpcErr.Code())
	})
}

func TestHTTPServiceRequestTimeout(t *testing.T) {
	completed := int64(0)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := pb.ConnectionResponse{
			Status: pb.Status_SUCCESS,
		}

		// Timers are not determenistic (especially on CI with OSX — don't know why)
		// let's make sure the request is slow enough to be cancelled
		for atomic.LoadInt64(&completed) == 0 {
			time.Sleep(50 * time.Millisecond)
		}

		w.Write(utils.ToJSON(&res)) // nolint: errcheck
	}))

	defer ts.Close()

	conf := NewConfig()
	conf.Host = ts.URL
	conf.RequestTimeout = 50

	service, _ := NewHTTPService(&conf)
	request := protocol.NewConnectMessage(
		common.NewSessionEnv("ws://anycable.io/cable", &map[string]string{"cookie": "foo=bar"}),
	)

	ctx := context.Background()

	_, err := service.Connect(ctx, request)
	atomic.AddInt64(&completed, 1)

	require.Error(t, err)

	grpcErr, ok := status.FromError(err)

	require.True(t, ok)

	assert.Equal(t, codes.DeadlineExceeded, grpcErr.Code())
}

func TestHTTPServiceBadRequests(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))

	defer ts.Close()

	conf := NewConfig()
	conf.Host = ts.URL
	conf.RequestTimeout = 50

	request := protocol.NewConnectMessage(
		common.NewSessionEnv("ws://anycable.io/cable", &map[string]string{"cookie": "foo=bar"}),
	)

	t.Run("unknown url", func(t *testing.T) {
		newConf := NewConfig()
		newConf.Host = "http://localhost:1234"

		service, _ := NewHTTPService(&newConf)

		ctx := context.Background()

		_, err := service.Connect(ctx, request)

		require.Error(t, err)

		grpcErr, ok := status.FromError(err)

		require.True(t, ok)

		assert.Equal(t, codes.Unavailable, grpcErr.Code())
	})

	t.Run("bad request", func(t *testing.T) {
		service, _ := NewHTTPService(&conf)

		ctx := context.Background()

		_, err := service.Connect(ctx, request)

		require.Error(t, err)

		grpcErr, ok := status.FromError(err)

		require.True(t, ok)

		assert.Equal(t, codes.InvalidArgument, grpcErr.Code())
	})
}

func TestHTTPClientHelper_READY(t *testing.T) {
	conf := NewConfig()
	conf.Host = "http://localhost:1234"

	service, _ := NewHTTPService(&conf)
	h := NewHTTPClientHelper(service)

	assert.NoError(t, h.Ready())

	// by default, we open a breaker if there are >20% of errors
	request := protocol.NewConnectMessage(
		common.NewSessionEnv("ws://anycable.io/cable", &map[string]string{"cookie": "foo=bar"}),
	)

	for i := 0; i < 20; i++ {
		_, err := service.Connect(context.Background(), request)
		require.Error(t, err)
	}

	// Shouldn't be ready
	assert.Error(t, h.Ready())
}
