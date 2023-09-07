package sse

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/utils"
	"github.com/joomcode/errorx"
)

func NewSSESession(n *node.Node, w http.ResponseWriter, info *server.RequestInfo) (*node.Session, error) {
	conn := NewConnection(w)

	session := node.NewSession(n, conn, info.URL, info.Headers, info.UID, node.WithEncoder(&Encoder{}))
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

	if identifier == "" {
		return nil, nil
	}

	msg.Identifier = identifier

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
