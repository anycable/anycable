package ocpp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const identifier = "{\"channel\":\"OCPPChannel\",\"sn\":\"ev2023\"}"

func TestEncoderEncode(t *testing.T) {
	coder := Encoder{}

	t.Run("Ping", func(t *testing.T) {
		msg := &common.PingMessage{Type: "ping", Message: time.Now().Unix()}
		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		// We ignore server-to-client pings and rely on client-to-server heartbeat messages
		assert.Nil(t, actual)
	})

	t.Run("Disconnect", func(t *testing.T) {
		msg := &common.DisconnectMessage{Type: "disconnect", Reason: "unauthorized", Reconnect: false}

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		// Not supported by the protocol, handled by the executor
		assert.Nil(t, actual)
	})

	t.Run("Ack", func(t *testing.T) {
		msg := &common.Reply{Identifier: "alai2022", Type: "Ack", Message: map[string]string{"status": "whoknows"}}

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, `[3,"alai2022",{"status":"whoknows"}]`, string(actual.Payload))
	})

	t.Run("Error", func(t *testing.T) {
		msg := &common.Reply{Identifier: "54", Type: "FormationViolation", Reason: "Already connected", Message: map[string]string{"context": "whateva"}}

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, `[4,"54","FormationViolation","Already connected",{"context":"whateva"}]`, string(actual.Payload))
	})

	t.Run("Broadcast", func(t *testing.T) {
		msg := &common.Reply{
			Identifier: IDToIdentifier("ev1", "MyOCPPChannel"),
			Message:    map[string]interface{}{"id": "we57", "command": "RemoteStopTransaction", "payload": map[string]string{"transactionId": "123"}},
		}

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, `[2,"we57","RemoteStopTransaction",{"transactionId":"123"}]`, string(actual.Payload))
	})
}

func TestEncoderEncodeTransmission(t *testing.T) {
	coder := Encoder{}

	t.Run("welcome", func(t *testing.T) {
		msg := "{\"type\":\"welcome\"}"

		actual, err := coder.EncodeTransmission(msg)

		require.NoError(t, err)
		assert.Nil(t, actual)
	})

	t.Run("ack", func(t *testing.T) {
		msg := toJSON(common.Reply{Identifier: identifier, Message: map[string]interface{}{"command": "Ack", "id": "x21", "payload": map[string]string{"status": "Accepted"}}})

		actual, err := coder.EncodeTransmission(string(msg))

		require.NoError(t, err)
		assert.Equal(t, `[3,"x21",{"status":"Accepted"}]`, string(actual.Payload))
	})

	t.Run("error", func(t *testing.T) {
		msg := toJSON(common.Reply{Identifier: identifier, Message: map[string]interface{}{"command": "Error", "id": "ce31", "error_code": "ER443", "error_message": "Something went wrong", "payload": map[string]string{"context": "whateva"}}})

		actual, err := coder.EncodeTransmission(string(msg))

		require.NoError(t, err)
		assert.Equal(t, `[4,"ce31","ER443","Something went wrong",{"context":"whateva"}]`, string(actual.Payload))
	})

	t.Run("message (with id)", func(t *testing.T) {
		msg := toJSON(common.Reply{Identifier: identifier, Message: map[string]interface{}{"id": "x21", "command": "TriggerMessage", "payload": map[string]string{"idTag": "42"}}})

		actual, err := coder.EncodeTransmission(string(msg))

		require.NoError(t, err)
		assert.Equal(t, `[2,"x21","TriggerMessage",{"idTag":"42"}]`, string(actual.Payload))
	})

	t.Run("message (without id)", func(t *testing.T) {
		msg := toJSON(common.Reply{Identifier: identifier, Message: map[string]interface{}{"command": "TriggerMessage", "payload": map[string]string{"idTag": "42"}}})

		actual, err := coder.EncodeTransmission(string(msg))

		require.NoError(t, err)

		payload := string(actual.Payload)
		assert.Contains(t, payload, `[2,`)
		assert.Contains(t, payload, `,"TriggerMessage",{"idTag":"42"}]`)
	})
}

func TestEncoderDecode(t *testing.T) {
	coder := Encoder{}

	t.Run("BootNotification", func(t *testing.T) {
		msg := `[2,"15","BootNotification",{"chargePointModel":"CPM","chargePointVendor":"CPV","chargePointSerialNumber":"CPSN","chargeBoxSerialNumber":"CBSN","firmwareVersion":"FV","iccid":"ICCID","imsi":"IMSI","meterType":"MT","meterSerialNumber":"MSN"}]`

		actual, err := coder.Decode([]byte(msg))

		require.NoError(t, err)
		assert.Equal(t, "15", actual.Identifier)
		assert.Equal(t, BootCommand, actual.Command)

		data, ok := actual.Data.(CallMessage)

		require.Truef(t, ok, "data is not CallMessage: %v", data)

		assert.Equal(t, map[string]interface{}{"chargePointModel": "CPM", "chargePointVendor": "CPV", "chargePointSerialNumber": "CPSN", "chargeBoxSerialNumber": "CBSN", "firmwareVersion": "FV", "iccid": "ICCID", "imsi": "IMSI", "meterType": "MT", "meterSerialNumber": "MSN"}, data.Payload)
	})

	t.Run("Accepted", func(t *testing.T) {
		msg := `[3,"44",{"status":"Accepted"}]`

		actual, err := coder.Decode([]byte(msg))

		require.NoError(t, err)
		assert.Equal(t, AckCommand, actual.Command)
		assert.Equal(t, "44", actual.Identifier)

		data, ok := actual.Data.(AckMessage)

		assert.True(t, ok)

		assert.Equal(t, map[string]interface{}{"status": "Accepted"}, data.Payload)
	})

	t.Run("Error", func(t *testing.T) {
		msg := `[4,"13","9338", "Doma byt' zaebis'", {"currentMusic":"Alai Oli"}]`

		actual, err := coder.Decode([]byte(msg))

		require.NoError(t, err)
		assert.Equal(t, ErrorCommand, actual.Command)
		assert.Equal(t, "13", actual.Identifier)

		data, ok := actual.Data.(ErrorMessage)

		assert.True(t, ok)

		assert.Equal(t, "9338", data.ErrorCode)
		assert.Equal(t, "Doma byt' zaebis'", data.ErrorDescription)
		assert.Equal(t, map[string]interface{}{"currentMusic": "Alai Oli"}, data.Payload)
	})

	t.Run("Error without code/description", func(t *testing.T) {
		msg := `[4,"13"]`

		actual, err := coder.Decode([]byte(msg))

		require.NoError(t, err)
		assert.Equal(t, ErrorCommand, actual.Command)
		assert.Equal(t, "13", actual.Identifier)

		data, ok := actual.Data.(ErrorMessage)

		assert.True(t, ok)

		assert.Equal(t, "", data.ErrorCode)
		assert.Equal(t, "", data.ErrorDescription)
		assert.Nil(t, data.Payload)
	})
}

func toJSON(msg interface{}) []byte {
	b, err := json.Marshal(&msg)
	if err != nil {
		panic("Failed to build JSON 😲")
	}

	return b
}
