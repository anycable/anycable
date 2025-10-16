package rpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"
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
	service, cleanup := runGRPCServer(t, 50051)
	defer cleanup()

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

		res, err := controller.Authenticate(context.Background(), "test", &common.SessionEnv{URL: "http://test.cable", Headers: &headers})

		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, common.SUCCESS, res.Status)

		err = controller.Disconnect(context.Background(), "test", &common.SessionEnv{URL: "http://test.cable", Headers: &headers}, "", []string{})
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

		err = controller.Disconnect(context.Background(), "test", &common.SessionEnv{URL: "http://test.cable", Headers: &headers}, "", []string{})

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

		res, err := controller.Perform(context.Background(), "test", &common.SessionEnv{URL: "http://test.cable", Headers: &headers}, "", "test", "data")

		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{"ok"}, res.Transmissions)

		assert.Equal(t, rpcCalls+1, metrics.Counter(metricsRPCCalls).Value())
		assert.Equal(t, rpcRetries+1, metrics.Counter(metricsRPCRetries).Value())
	})
}

func TestGRPCMultiHost(t *testing.T) {
	service, cleanup := runGRPCServer(t, 50151)
	defer cleanup()

	service2, cleanup2 := runGRPCServer(t, 50152)
	defer cleanup2()

	var service1_called int32
	var service2_called int32

	service.On("Connect", mock.Anything, mock.Anything).Return(&pb.ConnectionResponse{Status: pb.Status_SUCCESS}, nil).Run(func(args mock.Arguments) {
		atomic.AddInt32(&service1_called, 1)
	}).After(10 * time.Microsecond)

	service2.On("Connect", mock.Anything, mock.Anything).Return(&pb.ConnectionResponse{Status: pb.Status_SUCCESS}, nil).Run(func(args mock.Arguments) {
		atomic.AddInt32(&service2_called, 1)
	}).After(10 * time.Microsecond)

	m := metrics.NewMetrics(nil, 0, slog.Default())
	l := slog.Default()
	headers := map[string]string{"cookie": "token=secret;"}

	c := NewConfig()
	c.Host = "grpc-list://localhost:50151,localhost:50152"
	controller, err := NewController(m, &c, l)
	require.NoError(t, err)
	require.NoError(t, controller.Start())
	defer controller.Shutdown() //nolint:errcheck

	for i := 0; i < 10; i++ {
		res, err := controller.Authenticate(context.Background(), "test", &common.SessionEnv{URL: "http://test.cable", Headers: &headers})

		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, common.SUCCESS, res.Status)
	}

	assert.Greater(t, service1_called, int32(0))
	assert.Greater(t, service2_called, int32(0))
}

func runGRPCServer(t *testing.T, port int) (*mocks.RPCServer, func()) {
	server := grpc.NewServer(
		grpc.MaxConcurrentStreams(1),
	)
	service := mocks.RPCServer{}
	pb.RegisterRPCServer(server, &service)

	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", port)) // nolint: gosec
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	go func() {
		if err := server.Serve(listen); err != nil {
			t.Errorf("failed to serve: %v", err)
		}
	}()

	return &service, func() { server.GracefulStop() }
}
