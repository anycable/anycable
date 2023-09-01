package sse

import (
	"net/http"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/utils"
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
func subscribeCommandFromRequest(r *http.Request) (*common.Message, error) {
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
