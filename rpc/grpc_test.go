package rpc

import (
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	pb "github.com/anycable/anycable-go/protos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGRPCIntegration(t *testing.T) {
	server := grpc.NewServer(
		grpc.MaxConcurrentStreams(1),
	)
	service := mocks.RPCServer{}
	pb.RegisterRPCServer(server, &service)

	listen, err := net.Listen("tcp", ":50051") // nolint: gosec
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	defer server.GracefulStop()

	go func() {
		if err := server.Serve(listen); err != nil {
			t.Errorf("failed to serve: %v", err)
		}
	}()

	// Connect is fast and successful
	service.On("Connect", mock.Anything, mock.Anything).Return(&pb.ConnectionResponse{Status: pb.Status_SUCCESS}, nil)
	// Disconnect is slow (to test resource exhaustion and timeouts)
	service.On("Disconnect", mock.Anything, mock.Anything).Return(&pb.DisconnectResponse{Status: pb.Status_SUCCESS}, nil).After(1 * time.Second)
	// Command emulates exhaustion
	st := status.New(codes.ResourceExhausted, "Request limit exceeded.")
	service.On("Command", mock.Anything, mock.Anything).Return(nil, st.Err()).Once()
	service.On("Command", mock.Anything, mock.Anything).Return(&pb.CommandResponse{Status: pb.Status_SUCCESS, Transmissions: []string{"ok"}}, nil)

	metrics := metrics.NewMetrics(nil, 0, slog.Default())
	logger := slog.Default()
	headers := map[string]string{"cookie": "token=secret;"}

	t.Run("Success", func(t *testing.T) {
		config := NewConfig()
		controller, err := NewController(metrics, &config, logger)
		require.NoError(t, err)
		require.NoError(t, controller.Start())
		defer controller.Shutdown() // nolint: errcheck

		rpcCalls := metrics.Counter(metricsRPCCalls).Value()

		res, err := controller.Authenticate("test", &common.SessionEnv{URL: "http://test.cable", Headers: &headers})

		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, common.SUCCESS, res.Status)

		err = controller.Disconnect("test", &common.SessionEnv{URL: "http://test.cable", Headers: &headers}, "", []string{})
		require.NoError(t, err)

		assert.Equal(t, rpcCalls+2, metrics.Counter(metricsRPCCalls).Value())
	})

	t.Run("Timeout", func(t *testing.T) {
		config := NewConfig()
		config.RequestTimeout = 500
		config.CommandTimeout = 500
		controller, err := NewController(metrics, &config, logger)
		require.NoError(t, err)
		require.NoError(t, controller.Start())
		defer controller.Shutdown() // nolint: errcheck

		rpcCalls := metrics.Counter(metricsRPCCalls).Value()
		rpcTimeouts := metrics.Counter(metricsRPCTimeouts).Value()
		rpcErrors := metrics.Counter(metricsRPCFailures).Value()

		err = controller.Disconnect("test", &common.SessionEnv{URL: "http://test.cable", Headers: &headers}, "", []string{})

		require.Error(t, err, "deadline exceeded")

		assert.Equal(t, rpcCalls+1, metrics.Counter(metricsRPCCalls).Value())
		assert.Equal(t, rpcTimeouts+1, metrics.Counter(metricsRPCTimeouts).Value())
		assert.Equal(t, rpcErrors+1, metrics.Counter(metricsRPCFailures).Value())
	})

	t.Run("Exhausted", func(t *testing.T) {
		config := NewConfig()
		controller, err := NewController(metrics, &config, logger)
		require.NoError(t, err)
		require.NoError(t, controller.Start())
		defer controller.Shutdown() // nolint: errcheck

		rpcCalls := metrics.Counter(metricsRPCCalls).Value()
		rpcRetries := metrics.Counter(metricsRPCRetries).Value()

		res, err := controller.Perform("test", &common.SessionEnv{URL: "http://test.cable", Headers: &headers}, "", "test", "data")

		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{"ok"}, res.Transmissions)

		assert.Equal(t, rpcCalls+1, metrics.Counter(metricsRPCCalls).Value())
		assert.Equal(t, rpcRetries+1, metrics.Counter(metricsRPCRetries).Value())
	})
}
