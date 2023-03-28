package ocpp

const (
	Subprotocol16 = "ocpp1.6"

	CallCode  = 2
	AckCode   = 3
	ErrorCode = 4

	// List command that must be treated specially by executor
	BootCommand      = "BootNotification"
	HeartbeatCommand = "Heartbeat"

	// Custom command to distinguish between Ack and other types
	AckCommand   = "Ack"
	ErrorCommand = "Error"

	// Errors
	FormationViolationError = "FormationViolation"
	NotSupportedError       = "NotSupported"
	SecurityError           = "SecurityError"
	InternalError           = "InternalError"
	ProtocolError           = "ProtocolError"
)

func Subprotocols() []string {
	return []string{Subprotocol16}
}

type Message interface {
	GetID() string
	GetCode() int
	GetPayload() interface{}
}

type CallMessage struct {
	UniqID  string
	Command string
	Payload interface{}
}

var _ Message = (*CallMessage)(nil)

func (m CallMessage) GetID() string {
	return m.UniqID
}

func (m CallMessage) GetCode() int {
	return CallCode
}

func (m CallMessage) GetPayload() interface{} {
	return m.Payload
}

type AckMessage struct {
	UniqID  string
	Payload interface{}
}

var _ Message = (*AckMessage)(nil)

func (m AckMessage) GetID() string {
	return m.UniqID
}

func (m AckMessage) GetCode() int {
	return AckCode
}

func (m AckMessage) GetPayload() interface{} {
	return m.Payload
}

type ErrorMessage struct {
	UniqID           string
	ErrorCode        string
	ErrorDescription string
	Payload          interface{}
}

var _ Message = (*ErrorMessage)(nil)

func (m ErrorMessage) GetID() string {
	return m.UniqID
}

func (m ErrorMessage) GetCode() int {
	return ErrorCode
}

func (m ErrorMessage) GetPayload() interface{} {
	return m.Payload
}
