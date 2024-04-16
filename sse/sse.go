package sse

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/utils"
	"github.com/joomcode/errorx"
)

const (
	signedStreamParam   = "signed_stream"
	publicStreamParam   = "stream"
	signedStreamChannel = "$pubsub"
	turboStreamsParam   = "turbo_signed_stream_name"
	turboStreamsChannel = "Turbo::StreamsChannel"
	historySinceParam   = "history_since"
)

func NewSSESession(n *node.Node, w http.ResponseWriter, r *http.Request, info *server.RequestInfo) (*node.Session, error) {
	conn := NewConnection(w)

	unwrapData := r.Method == http.MethodGet

	session := node.NewSession(n, conn, info.URL, info.Headers, info.UID, node.WithEncoder(&Encoder{unwrapData}))
	res, err := n.Authenticate(session)

	if err != nil {
		return nil, err
	}

	if res.Status == common.SUCCESS {
		return session, nil
	} else {
		return nil, nil
	}
}

// Extract channel identifier or name from the request and build a subscribe command payload
func subscribeCommandsFromRequest(r *http.Request) ([]*common.Message, error) {
	if r.Method == http.MethodGet {
		cmd, err := subscribeCommandFromGetRequest(r)

		if err != nil {
			return nil, err
		}

		if cmd == nil {
			return nil, errors.New("no channel provided")
		}

		return []*common.Message{cmd}, nil

	} else {
		return subscribeCommandFromPostRequest(r)
	}
}

func subscribeCommandFromGetRequest(r *http.Request) (*common.Message, error) {
	msg := &common.Message{
		Command: "subscribe",
	}

	// First, check if identifier is provided
	identifier := r.URL.Query().Get("identifier")

	if identifier == "" {
		channel := r.URL.Query().Get("channel")

		if channel != "" {
			identifier = string(utils.ToJSON(map[string]string{"channel": channel}))
		}
	}

	// Check for public stream name
	if identifier == "" {
		stream := r.URL.Query().Get(publicStreamParam)

		if stream != "" {
			identifier = string(utils.ToJSON(map[string]string{
				"channel":     signedStreamChannel,
				"stream_name": stream,
			}))
		}
	}

	// Check for signed stream name
	if identifier == "" {
		stream := r.URL.Query().Get(signedStreamParam)

		if stream != "" {
			identifier = string(utils.ToJSON(map[string]string{
				"channel":            signedStreamChannel,
				"signed_stream_name": stream,
			}))
		}
	}

	// Then, check for Turbo Streams name
	if identifier == "" {
		stream := r.URL.Query().Get(turboStreamsParam)

		if stream != "" {
			identifier = string(utils.ToJSON(map[string]string{
				"channel":            turboStreamsChannel,
				"signed_stream_name": stream,
			}))
		}
	}

	if identifier == "" {
		return nil, nil
	}

	msg.Identifier = identifier

	if lastId := r.Header.Get("last-event-id"); lastId != "" {
		offsetParts := strings.SplitN(lastId, lastIdDelimeter, 3)

		if len(offsetParts) == 3 {
			offset, err := strconv.ParseUint(offsetParts[0], 10, 64)

			if err != nil {
				return nil, errorx.Decorate(err, "failed to parse last event id: %s", lastId)
			}

			epoch := offsetParts[1]
			stream := offsetParts[2]

			streams := make(map[string]common.HistoryPosition)

			streams[stream] = common.HistoryPosition{Offset: offset, Epoch: epoch}

			msg.History = common.HistoryRequest{
				Streams: streams,
			}
		}
	}

	if since := r.URL.Query().Get(historySinceParam); since != "" {
		sinceInt, err := strconv.ParseInt(since, 10, 64)
		if err != nil {
			return nil, errorx.Decorate(err, "failed to parse history since value: %s", since)
		}

		msg.History.Since = sinceInt
	}

	return msg, nil
}

func subscribeCommandFromPostRequest(r *http.Request) ([]*common.Message, error) {
	var cmds []*common.Message

	// Read commands (if any)
	if r.Body != nil {
		r.Body = http.MaxBytesReader(nil, r.Body, int64(defaultMaxBodySize))
		requestData, err := io.ReadAll(r.Body)

		if err != nil {
			return nil, err
		}

		if len(requestData) > 0 {
			lines := bytes.Split(requestData, []byte("\n"))

			for _, line := range lines {
				if len(line) > 0 {
					var command common.Message
					err := json.Unmarshal(line, &command)

					if err != nil {
						return nil, errorx.Decorate(err, "failed to parse command: %v", command)
					}

					cmds = append(cmds, &command)
				}
			}
		}
	}

	return cmds, nil
}
