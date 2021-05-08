package encoders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockMessage struct {
	Encoded int
	Value   string
}

func (m *MockMessage) ToJSON() []byte {
	m.Encoded++
	return []byte(m.Value)
}

func (m *MockMessage) GetType() string {
	return "mock"
}

func TestCachedEncodedMessage(t *testing.T) {
	msg := &MockMessage{Value: "mock"}

	cm := NewCachedEncodedMessage(msg)

	assert.Equal(t, []byte("mock"), cm.ToJSON())
	assert.Equal(t, []byte("mock"), cm.ToJSON())
	assert.Equal(t, 1, msg.Encoded)
}
