package kinesis

import (
	"encoding/base64"
	"encoding/json"

	"github.com/apex/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/rogpeppe/fastuuid"
	k "github.com/tj/go-kinesis"
)

// Handler implementation.
type Handler struct {
	appName  string
	producer *k.Producer
	gen      *fastuuid.Generator
}

// New handler sending logs to Kinesis. To configure producer options or pass your
// own AWS Kinesis client use NewConfig instead.
func New(stream string) *Handler {
	return NewConfig(k.Config{
		StreamName: stream,
		Client:     kinesis.New(session.New(aws.NewConfig())),
	})
}

// NewConfig handler sending logs to Kinesis. The `config` given is passed to the batch
// Kinesis producer, and a random value is used as the partition key for even distribution.
func NewConfig(config k.Config) *Handler {
	producer := k.New(config)
	producer.Start()
	return &Handler{
		producer: producer,
		gen:      fastuuid.MustNewGenerator(),
	}
}

// HandleLog implements log.Handler.
func (h *Handler) HandleLog(e *log.Entry) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}

	uuid := h.gen.Next()
	key := base64.StdEncoding.EncodeToString(uuid[:])
	return h.producer.Put(b, key)
}
