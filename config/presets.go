package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"

	gonanoid "github.com/matoous/go-nanoid"
)

func (c *Config) Presets() []string {
	if c.UserPresets != nil {
		return c.UserPresets
	}

	return detectPresetsFromEnv()
}

func (c *Config) LoadPresets(logger *slog.Logger) error {
	presets := c.Presets()

	if len(presets) == 0 {
		return nil
	}

	logger.With("context", "config").Info("load presets", "presets", strings.Join(presets, ","))

	defaults := NewConfig()

	for _, preset := range presets {
		switch preset {
		case "fly":
			if err := c.loadFlyPreset(&defaults); err != nil {
				return err
			}
		case "heroku":
			if err := c.loadHerokuPreset(&defaults); err != nil {
				return err
			}
		case "broker":
			if err := c.loadBrokerPreset(&defaults); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Config) loadFlyPreset(defaults *Config) error {
	if c.Server.Host == defaults.Server.Host {
		c.Server.Host = "0.0.0.0"
	}

	region, ok := os.LookupEnv("FLY_REGION")

	if !ok {
		return errors.New("FLY_REGION env is missing")
	}

	appName, ok := os.LookupEnv("FLY_APP_NAME")

	if !ok {
		return errors.New("FLY_APP_NAME env is missing")
	}

	appId, _ := os.LookupEnv("FLY_ALLOC_ID")

	redisEnabled := (c.Redis.URL != defaults.Redis.URL)

	// Use the same port for HTTP broadcasts by default
	if c.HTTPBroadcast.Port == defaults.HTTPBroadcast.Port {
		c.HTTPBroadcast.Port = c.Server.Port
	}

	if c.EmbeddedNats.Name == defaults.EmbeddedNats.Name && appId != "" {
		// We need to be unique to avoid having duplicates during deployments
		suf, _ := gonanoid.Nanoid(3) // nolint: errcheck
		c.EmbeddedNats.Name = fmt.Sprintf("fly-%s-%s-%s", region, appId, suf)
	}

	if c.EmbeddedNats.ServiceAddr == defaults.EmbeddedNats.ServiceAddr {
		c.EmbeddedNats.ServiceAddr = "nats://0.0.0.0:4222"
	}

	if c.EmbeddedNats.ClusterAddr == defaults.EmbeddedNats.ClusterAddr {
		c.EmbeddedNats.ClusterAddr = "nats://0.0.0.0:5222"
	}

	if c.EmbeddedNats.ClusterName == defaults.EmbeddedNats.ClusterName {
		c.EmbeddedNats.ClusterName = fmt.Sprintf("%s-%s-cluster", appName, region)
	}

	if c.EmbeddedNats.Routes == nil {
		c.EmbeddedNats.Routes = []string{fmt.Sprintf("nats://%s.%s.internal:5222", region, appName)}
	}

	if c.EmbeddedNats.GatewayAdvertise == defaults.EmbeddedNats.GatewayAdvertise {
		c.EmbeddedNats.GatewayAdvertise = fmt.Sprintf("%s.%s.internal:7222", region, appName)
	}

	// Enable embedded NATS by default unless another adapter is set for PubSub
	// or Redis URL is provided
	if c.PubSubAdapter == defaults.PubSubAdapter {
		if redisEnabled {
			c.PubSubAdapter = "redis"
		} else {
			c.PubSubAdapter = "nats"

			// NATS hasn't been configured, so we can embed it
			if !c.EmbeddedNats.Enabled || c.NATS.Servers == defaults.NATS.Servers {
				c.EmbeddedNats.Enabled = true
				c.NATS.Servers = c.EmbeddedNats.ServiceAddr
				c.BroadcastAdapters = []string{"http", "nats"}
			}
		}
	}

	if !redisEnabled && slices.Equal(c.BroadcastAdapters, defaults.BroadcastAdapters) {
		c.BroadcastAdapters = []string{"http"}
	}

	if rpcName, ok := os.LookupEnv("ANYCABLE_FLY_RPC_APP_NAME"); ok {
		if c.RPC.Host == defaults.RPC.Host {
			c.RPC.Host = fmt.Sprintf("dns:///%s.%s.internal:50051", region, rpcName)
		}
	}

	return nil
}

func (c *Config) loadHerokuPreset(defaults *Config) error {
	if c.Server.Host == defaults.Server.Host {
		c.Server.Host = "0.0.0.0"
	}

	if c.HTTPBroadcast.Port == defaults.HTTPBroadcast.Port {
		if herokuPortStr := os.Getenv("PORT"); herokuPortStr != "" {
			herokuPort, err := strconv.Atoi(herokuPortStr)
			if err != nil {
				return err
			}

			c.HTTPBroadcast.Port = herokuPort
		}
	}

	return nil
}

func (c *Config) loadBrokerPreset(defaults *Config) error {
	redisEnabled := (c.Redis.URL != defaults.Redis.URL)
	enatsEnabled := c.EmbeddedNats.Enabled

	if c.Broker.Adapter == defaults.Broker.Adapter {
		if enatsEnabled {
			c.Broker.Adapter = "nats"
		} else {
			c.Broker.Adapter = "memory"
		}
	}

	if slices.Equal(c.BroadcastAdapters, defaults.BroadcastAdapters) {
		switch {
		case enatsEnabled:
			c.BroadcastAdapters = []string{"http", "nats"}
		case redisEnabled:
			c.BroadcastAdapters = []string{"http", "redisx", "redis"}
		default:
			c.BroadcastAdapters = []string{"http"}
		}
	}

	if c.PubSubAdapter == defaults.PubSubAdapter {
		switch {
		case enatsEnabled:
			c.PubSubAdapter = "nats"
		case redisEnabled:
			c.PubSubAdapter = "redis"
		}
	}

	return nil
}

func detectPresetsFromEnv() []string {
	presets := []string{}

	if isFlyEnv() {
		presets = append(presets, "fly")
	}

	if isHerokuEnv() {
		presets = append(presets, "heroku")
	}

	return presets
}

func isFlyEnv() bool {
	if _, ok := os.LookupEnv("FLY_APP_NAME"); !ok {
		return false
	}

	if _, ok := os.LookupEnv("FLY_ALLOC_ID"); !ok {
		return false
	}

	if _, ok := os.LookupEnv("FLY_REGION"); !ok {
		return false
	}

	return true
}

func isHerokuEnv() bool {
	if _, ok := os.LookupEnv("HEROKU_APP_ID"); !ok {
		return false
	}

	if _, ok := os.LookupEnv("HEROKU_DYNO_ID"); !ok {
		return false
	}

	return true
}
