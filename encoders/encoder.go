package encoders

import (
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"
)

type EncodedMessage interface {
	GetType() string
}

var _ EncodedMessage = (*common.Reply)(nil)
var _ EncodedMessage = (*common.PingMessage)(nil)
var _ EncodedMessage = (*common.DisconnectMessage)(nil)

type Encoder interface {
	ID() string
	Encode(msg EncodedMessage) (*ws.SentFrame, error)
	EncodeTransmission(msg string) (*ws.SentFrame, error)
	Decode(payload []byte) (*common.Message, error)
}

var _ Encoder = (*JSON)(nil)
var _ Encoder = (*Msgpack)(nil)
