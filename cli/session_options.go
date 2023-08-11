package cli

import (
	"strconv"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/graphql"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/ocpp"
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

	if common.IsMsgpackProtocol(protocol) {
		opts = append(opts, node.WithEncoder(encoders.Msgpack{}))
	}

	if common.IsProtobufProtocol(protocol) {
		opts = append(opts, node.WithEncoder(encoders.Protobuf{}))
	}

	if protocol == graphql.GraphqlWsProtocol {
		opts = append(
			opts,
			node.WithEncoder(graphql.Encoder{}),
			node.WithExecutor(graphql.NewExecutor(r.node, &r.config.GraphQL)),
			node.WithHandshakeMessageDeadline(time.Now().Add(time.Duration(r.config.GraphQL.IdleTimeout)*time.Second)),
		)
	}

	if protocol == graphql.LegacyGraphQLProtocol {
		opts = append(
			opts,
			node.WithEncoder(graphql.LegacyEncoder{}),
			node.WithExecutor(graphql.NewLegacyExecutor(r.node, &r.config.GraphQL)),
			node.WithHandshakeMessageDeadline(time.Now().Add(time.Duration(r.config.GraphQL.IdleTimeout)*time.Second)),
		)
	}

	if protocol == ocpp.Subprotocol16 {
		opts = append(
			opts,
			node.WithEncoder(ocpp.Encoder{}),
			node.WithExecutor(ocpp.NewExecutor(r.node, &r.config.OCPP)),
			node.WithHandshakeMessageDeadline(time.Now().Add(time.Duration(r.config.OCPP.IdleTimeout)*time.Second)),
		)
	}

	if r.config.App.EnableNativePing && r.config.App.PongTimeout > 0 {
		opts = append(opts, node.WithPongTimeout(time.Duration(r.config.App.PongTimeout)*time.Second))
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
	opts = append(opts, node.WithMaxPendingSize(c.MaxPendingSize))
	return opts
}
