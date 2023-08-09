package cli

import (
	"strconv"
	"time"

	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/apex/log"
)

const (
	pingIntervalParameter  = "pi"
	pingPrecisionParameter = "ptp"
)

func (r *Runner) sessionOptionsFromProtocol(protocol string) []node.SessionOption {
	opts := []node.SessionOption{}

	return opts
}

func (r *Runner) sessionOptionsFromParams(info *server.RequestInfo) []node.SessionOption {
	opts := []node.SessionOption{}

	if rawVal := info.Param(pingIntervalParameter); rawVal != "" {
		val, err := strconv.Atoi(rawVal)
		if err != nil {
			log.Warnf("Invalid ping interval value, must be integer: %s", rawVal)
		} else {
			opts = append(opts, node.WithPingInterval(time.Duration(val)*time.Second))
		}
	}

	if val := info.Param(pingPrecisionParameter); val != "" {
		opts = append(opts, node.WithPingPrecision(val))
	}

	return opts
}
