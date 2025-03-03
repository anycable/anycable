package cli

import (
	"strconv"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/ws"
)

const (
	pingIntervalParameter  = "pi"
	pingPrecisionParameter = "ptp"

	prevSessionHeader = "X-ANYCABLE-RESTORE-SID"
	prevSessionParam  = "sid"
)

func (r *Runner) sessionOptionsFromProtocol(protocol string) []node.SessionOption {
	opts := []node.SessionOption{}

	if common.IsExtendedActionCableProtocol(protocol) {
		// configure session cache and presence keepalive intervals
		// (must be slightly less than the corresponding TTLs)
		opts = append(opts,
			node.WithKeepaliveIntervals(
				time.Duration(r.config.Broker.SessionsTTL*500)*time.Millisecond,
				time.Duration(r.config.Broker.PresenceTTL*500)*time.Millisecond,
			),
		)

		if r.config.App.PongTimeout > 0 {
			opts = append(opts, node.WithPongTimeout(time.Duration(r.config.App.PongTimeout)*time.Second))
		}
	}

	return opts
}

func (r *Runner) sessionOptionsFromParams(info *server.RequestInfo) []node.SessionOption {
	opts := []node.SessionOption{}

	if rawVal := info.Param(pingIntervalParameter); rawVal != "" {
		val, err := strconv.Atoi(rawVal)
		if err != nil {
			r.log.Warn("invalid ping interval value, must be integer", "val", rawVal)
		} else {
			opts = append(opts, node.WithPingInterval(time.Duration(val)*time.Second))
		}
	}

	if val := info.Param(pingPrecisionParameter); val != "" {
		opts = append(opts, node.WithPingPrecision(val))
	}

	if hval := info.AnyCableHeader(prevSessionHeader); hval != "" {
		opts = append(opts, node.WithPrevSID(hval))
	} else if pval := info.Param(prevSessionParam); pval != "" {
		opts = append(opts, node.WithPrevSID(pval))
	}

	return opts
}

func (r *Runner) sessionOptionsFromWSConfig(c *ws.Config) []node.SessionOption {
	opts := []node.SessionOption{}

	opts = append(opts, node.WithWriteTimeout(time.Duration(c.WriteTimeout)*time.Second))
	return opts
}
