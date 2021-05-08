package encoders

import (
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"
)

type EncodedMessage interface {
	GetType() string
	ToJSON() []byte
}

var _ EncodedMessage = (*common.Reply)(nil)
var _ EncodedMessage = (*common.PingMessage)(nil)
var _ EncodedMessage = (*common.DisconnectMessage)(nil)

type CachedEncodedMessage struct {
	target EncodedMessage

	// Encoder type to encoded message mapping
	encodedBytes map[string][]byte
}

var _ EncodedMessage = (*CachedEncodedMessage)(nil)

type Encoder interface {
	Encode(msg EncodedMessage) (*ws.SentFrame, error)
	EncodeTransmission(msg string) (*ws.SentFrame, error)
	Decode(payload []byte) (*common.Message, error)
}

var _ Encoder = (*JSON)(nil)

func NewCachedEncodedMessage(target EncodedMessage) *CachedEncodedMessage {
	return &CachedEncodedMessage{target, make(map[string][]byte)}
}

// ToJSON calls the target's ToJSON only once, memoizes the result
// and returns for the subsequent calls
func (m *CachedEncodedMessage) ToJSON() []byte {
	if _, ok := m.encodedBytes[jsonEncoderID]; !ok {
		m.encodedBytes[jsonEncoderID] = m.target.ToJSON()
	}

	return m.encodedBytes[jsonEncoderID]
}

func (m *CachedEncodedMessage) GetType() string {
	return m.target.GetType()
}
