package encoders

import (
	"encoding/json"

	"github.com/anycable/anycable-go/ws"
	"github.com/joomcode/errorx"
)

type EncodingCache struct {
	// Encoder type to encoded message mapping
	encodedBytes  map[string]*ws.SentFrame
	encoderErrors map[string]error
}

type EncodingFunction = func(EncodedMessage) (*ws.SentFrame, error)

func NewEncodingCache() *EncodingCache {
	return &EncodingCache{make(map[string]*ws.SentFrame), make(map[string]error)}
}

func (m *EncodingCache) Fetch(
	msg EncodedMessage,
	encoder string,
	callback EncodingFunction,
) (*ws.SentFrame, error) {
	if _, ok := m.encodedBytes[encoder]; !ok {
		b, err := callback(msg)

		if err != nil {
			m.encodedBytes[encoder] = nil
			m.encoderErrors[encoder] = err
		} else {
			m.encodedBytes[encoder] = b
		}
	}

	if b := m.encodedBytes[encoder]; b == nil {
		return nil, errorx.Decorate(m.encoderErrors[encoder], "encoding failed")
	} else {
		return b, nil
	}
}

type CachedEncodedMessage struct {
	target EncodedMessage
	cache  *EncodingCache
}

func NewCachedEncodedMessage(msg EncodedMessage) *CachedEncodedMessage {
	return &CachedEncodedMessage{target: msg, cache: NewEncodingCache()}
}

func (msg *CachedEncodedMessage) GetType() string {
	return msg.target.GetType()
}

func (msg *CachedEncodedMessage) Fetch(id string, callback EncodingFunction) (*ws.SentFrame, error) {
	return msg.cache.Fetch(msg.target, id, callback)
}

func (msg *CachedEncodedMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(msg.target)
}
