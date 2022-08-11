package encoders

import (
	"encoding/json"
	"fmt"

	"github.com/golang/protobuf/proto" // nolint:staticcheck
	"github.com/vmihailenco/msgpack/v5"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"

	pb "github.com/anycable/anycable-go/ac_protos"
)

const protobufEncoderID = "protobuf"

type Protobuf struct {
}

func (Protobuf) ID() string {
	return protobufEncoderID
}

func (Protobuf) Encode(msg EncodedMessage) (*ws.SentFrame, error) {
	buf := &pb.Message{}

	var err error
	var payload []byte

	if msg.GetType() == common.PingType {
		buf.Type = pb.Type_ping

		payload, err = msgpack.Marshal(msg.(*common.PingMessage).Message)
		if err != nil {
			return nil, err
		}

		goto END
	}

	if msg.GetType() == common.DisconnectType {
		if disconnect, ok := msg.(*common.DisconnectMessage); ok {
			buf.Type = pb.Type_disconnect
			buf.Reason = disconnect.Reason
			buf.Reconnect = disconnect.Reconnect

			goto END
		}
	}

	if msg.GetType() == common.WelcomeType {
		buf.Type = pb.Type_welcome

		goto END
	}

	if reply, ok := msg.(*common.Reply); ok {
		buf.Identifier = reply.Identifier

		if reply.Type == common.ConfirmedType {
			buf.Type = pb.Type_confirm_subscription
		}

		if reply.Type == common.RejectedType {
			buf.Type = pb.Type_reject_subscription
		}

		// Disconnect could be send either directly by server or via RPC,
		// so we need to handle it here as well
		if reply.Type == common.DisconnectType {
			buf.Type = pb.Type_disconnect
			buf.Reason = reply.Reason
			buf.Reconnect = reply.Reconnect
		}

		if reply.Message != nil {
			var mbytes []byte
			mbytes, err = msgpack.Marshal(&reply.Message)
			if err != nil {
				return nil, err
			}

			buf.Message = mbytes
		}
	} else {
		return nil, fmt.Errorf("Unknown message type: %v", msg)
	}

END:
	if payload != nil {
		buf.Message = payload
	}

	b, err := proto.Marshal(buf)
	if err != nil {
		panic("Failed to build protobuf 😲")
	}
	return &ws.SentFrame{FrameType: ws.BinaryFrame, Payload: b}, nil
}

func (p Protobuf) EncodeTransmission(raw string) (*ws.SentFrame, error) {
	msg := common.Reply{}

	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return nil, err
	}

	return p.Encode(&msg)
}

func (Protobuf) Decode(raw []byte) (*common.Message, error) {
	buf := &pb.Message{}
	if err := proto.Unmarshal(raw, buf); err != nil {
		return nil, err
	}

	msg := common.Message{}

	msg.Command = buf.Command.String()
	msg.Identifier = buf.Identifier
	msg.Data = buf.Data

	return &msg, nil
}
