package ws

import "github.com/gorilla/websocket"

const (
	// CloseNormalClosure indicates normal closure
	CloseNormalClosure = websocket.CloseNormalClosure

	// CloseInternalServerErr indicates closure because of internal error
	CloseInternalServerErr = websocket.CloseInternalServerErr

	// CloseAbnormalClosure indicates abnormal close
	CloseAbnormalClosure = websocket.CloseAbnormalClosure

	// CloseGoingAway indicates closing because of server shuts down or client disconnects
	CloseGoingAway = websocket.CloseGoingAway
)

var (
	expectedCloseStatuses = []int{
		websocket.CloseNormalClosure,    // Reserved in case ActionCable fixes its behaviour
		websocket.CloseGoingAway,        // Web browser page was closed
		websocket.CloseNoStatusReceived, // ActionCable don't care about closing
	}
)

type FrameType int

const (
	TextFrame   FrameType = 0
	CloseFrame  FrameType = 1
	BinaryFrame FrameType = 2
)

type SentFrame struct {
	FrameType   FrameType
	Payload     []byte
	CloseCode   int
	CloseReason string
}

func IsCloseError(err error) bool {
	return websocket.IsCloseError(err, expectedCloseStatuses...)
}
