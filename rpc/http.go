package rpc

import (
	"context"

	pb "github.com/anycable/anycable-go/protos"
)

type httpClientHelper struct {
	service *HTTPService
}

func NewHTTPClientHelper(s *HTTPService) *httpClientHelper {
	return &httpClientHelper{service: s}
}

func (h *httpClientHelper) Ready() error {
	return nil
}

func (h *httpClientHelper) SupportsActiveConns() bool {
	return false
}

func (h *httpClientHelper) ActiveConns() int {
	return 0
}

func (h *httpClientHelper) Close() {

}

type HTTPService struct {
	conf *Config
}

func NewHTTPDialer(c *Config) Dialer {
	service := NewHTTPService(c)
	helper := NewHTTPClientHelper(service)

	return NewInprocessServiceDialer(service, helper)
}

func NewHTTPService(c *Config) *HTTPService {
	return &HTTPService{conf: c}
}

func (s *HTTPService) Connect(ctx context.Context, r *pb.ConnectionRequest) (*pb.ConnectionResponse, error) {
	return &pb.ConnectionResponse{
		Status:        pb.Status_SUCCESS,
		Transmissions: []string{`{"type":"welcome"}`},
	}, nil
}

func (s *HTTPService) Disconnect(ctx context.Context, r *pb.DisconnectRequest) (*pb.DisconnectResponse, error) {
	return &pb.DisconnectResponse{
		Status: pb.Status_SUCCESS,
	}, nil
}

func (s *HTTPService) Command(ctx context.Context, r *pb.CommandMessage) (*pb.CommandResponse, error) {
	return nil, nil
}
