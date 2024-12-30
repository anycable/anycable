package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/anycable/anycable-go/logger"
	pb "github.com/anycable/anycable-go/protos"
	"github.com/anycable/anycable-go/utils"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type httpClientHelper struct {
	service *HTTPService
}

func NewHTTPClientHelper(s *HTTPService) *httpClientHelper {
	return &httpClientHelper{service: s}
}

func (h *httpClientHelper) Ready() error {
	cbState := h.service.cb.State()

	if cbState == gobreaker.StateOpen {
		return errors.New("http rpc is temporarily unavailable")
	}

	return nil
}

func (h *httpClientHelper) SupportsActiveConns() bool {
	return false
}

func (h *httpClientHelper) ActiveConns() int {
	return 0
}

func (h *httpClientHelper) Close() {
	h.service.client.CloseIdleConnections()
}

type HTTPService struct {
	conf    *Config
	client  *http.Client
	baseURL *url.URL

	cb *gobreaker.TwoStepCircuitBreaker

	pb.UnimplementedRPCServer
}

var _ pb.RPCServer = (*HTTPService)(nil)

func NewHTTPDialer(c *Config) (Dialer, error) {
	service, err := NewHTTPService(c)

	if err != nil {
		return nil, err
	}

	helper := NewHTTPClientHelper(service)

	return NewInprocessServiceDialer(service, helper), nil
}

func NewHTTPService(c *Config) (*HTTPService, error) {
	tlsConfig, error := c.TLSConfig()
	if error != nil {
		return nil, error
	}

	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}

	baseURL, err := url.Parse(c.Host)

	if err != nil {
		return nil, err
	}

	cb := gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{
		Name:        "httrpc",
		MaxRequests: 5,
		Interval:    10 * time.Second,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.8
		},
	})

	return &HTTPService{conf: c, client: client, baseURL: baseURL, cb: cb}, nil
}

func (s *HTTPService) Connect(ctx context.Context, r *pb.ConnectionRequest) (*pb.ConnectionResponse, error) {
	rawResponse, err := s.performRequest(ctx, "connect", utils.ToJSON(r))

	if err != nil {
		return nil, err
	}

	var response pb.ConnectionResponse

	err = json.Unmarshal(rawResponse, &response)

	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *HTTPService) Disconnect(ctx context.Context, r *pb.DisconnectRequest) (*pb.DisconnectResponse, error) {
	rawResponse, err := s.performRequest(ctx, "disconnect", utils.ToJSON(r))

	if err != nil {
		return nil, err
	}

	var response pb.DisconnectResponse

	err = json.Unmarshal(rawResponse, &response)

	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *HTTPService) Command(ctx context.Context, r *pb.CommandMessage) (*pb.CommandResponse, error) {
	rawResponse, err := s.performRequest(ctx, "command", utils.ToJSON(r))

	if err != nil {
		return nil, err
	}

	var response pb.CommandResponse

	err = json.Unmarshal(rawResponse, &response)

	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *HTTPService) performRequest(ctx context.Context, path string, payload []byte) ([]byte, error) {
	cbCallback, err := s.cb.Allow()

	if err != nil {
		return nil, err
	}

	url := s.baseURL.JoinPath(path).String()

	// We use timeouts to detect request queueing at the HTTP RPC side and report ResourceExhausted errors
	// (so adaptive concurrency control can be applied)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.conf.RequestTimeout)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if s.conf.Secret != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.conf.Secret))
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// Set headers from metadata
		for k, v := range md {
			req.Header.Set(fmt.Sprintf("x-anycable-meta-%s", k), v[0])
		}
	}

	res, err := s.client.Do(req)

	if err != nil {
		if ctx.Err() != nil {
			return nil, status.Error(codes.DeadlineExceeded, "request timeout")
		}

		cbCallback(false)
		return nil, status.Error(codes.Unavailable, err.Error())
	}

	cbCallback(true)

	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized {
		return nil, status.Error(codes.Unauthenticated, "http returned 401")
	}

	if res.StatusCode == http.StatusBadRequest || res.StatusCode == http.StatusUnprocessableEntity {
		reason, rerr := io.ReadAll(res.Body)
		if rerr != nil {
			return nil, status.Error(codes.InvalidArgument, "unprocessable entity")
		}

		return nil, status.Error(codes.InvalidArgument, logger.CompactValue(reason).String())
	}

	if res.StatusCode != http.StatusOK {
		reason, rerr := io.ReadAll(res.Body)
		if rerr != nil {
			return nil, status.Error(codes.Unknown, "internal error")
		}

		return nil, status.Error(codes.Unknown, logger.CompactValue(reason).String())
	}

	// Finally, the response is successful, let's read the body
	rawRequest, err := io.ReadAll(res.Body)

	if err != nil {
		return nil, err
	}

	return rawRequest, nil
}
