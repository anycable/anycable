package encoders

import (
	"encoding/json"
	"errors"

	"github.com/anycable/anycable-go/ws"
)

type EncodingCache struct {
	// Encoder type to encoded message mapping
	encodedBytes map[string]*ws.SentFrame
}

type EncodingFunction = func(EncodedMessage) (*ws.SentFrame, error)

func NewEncodingCache() *EncodingCache {
	return &EncodingCache{make(map[string]*ws.SentFrame)}
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
		} else {
			m.encodedBytes[encoder] = b
		}
	}

	if b := m.encodedBytes[encoder]; b == nil {
		return nil, errors.New("Encoding failed")
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
