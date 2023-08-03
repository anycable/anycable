//go:build freebsd && !amd64
// +build freebsd,!amd64

package enats

import (
	"context"
	"errors"
)

// NewConfig returns defaults for NATSServiceConfig
func NewConfig() Config {
	return Config{}
}

type Service struct{}

func (Service) Description() string { return "" }
func (Service) Start() error {
	return errors.New("embedded NATS is not supported for the current platform")
}
func (Service) Shutdown(ctx context.Context) error { return nil }

func NewService(c *Config) *Service {
	return &Service{}
}
