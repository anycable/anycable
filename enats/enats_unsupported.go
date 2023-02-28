//go:build freebsd && !amd64
// +build freebsd,!amd64

package enats

import (
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
func (Service) Shutdown() error { return nil }

func NewService(c *Config) *Service {
	return &Service{}
}
