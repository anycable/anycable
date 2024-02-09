package cli

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
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
		opts = append(opts, node.WithResumable(true))

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
			slog.Warn("invalid ping interval value, must be integer", "val", rawVal)
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
