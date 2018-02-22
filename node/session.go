package node

import (
	"net/http"
	"sync"
	"time"

	"github.com/anycable/anycable-go/utils"
	"github.com/apex/log"
	"github.com/gorilla/websocket"
)

const (
	// DefaultCloseStatus is what it states)
	DefaultCloseStatus = 3000

	// WriteWait is a time allowed to write a message to the peer.
	WriteWait = 10 * time.Second

	// MaxMessageSize is a max message size allowed from peer.
	MaxMessageSize = 512
)

// Session represents active client
type Session struct {
	node          *Node
	ws            *websocket.Conn
	path          string
	headers       map[string]string
	uid           string
	identifiers   string
	subscriptions map[string]bool
	send          chan []byte
	closed        bool
	connected     bool
	mu            sync.Mutex
	Log           *log.Entry
}

// NewSession build a new Session struct from ws connetion and http request
func NewSession(node *Node, ws *websocket.Conn, request *http.Request) (*Session, error) {
	path := request.URL.String()
	headers := utils.FetchHeaders(request, node.Config.Headers)

	session := &Session{
		node:          node,
		ws:            ws,
		path:          path,
		headers:       headers,
		subscriptions: make(map[string]bool),
		send:          make(chan []byte, 256),
		closed:        false,
		connected:     false,
	}

	identifiers, err := node.Authenticate(path, headers)

	if err != nil {
		defer session.Close("Auth Error")
		return nil, err
	}

	session.uid = "random-uid"

	ctx := log.WithFields(log.Fields{
		"sid": session.UID(),
	})

	session.Log = ctx
	session.identifiers = identifiers
	session.connected = true

	go session.SendMessages()

	return session, nil
}

// SendMessages waits for incoming messages and send them to the client connection
func (s *Session) SendMessages() {
	defer s.Disconnect("Write Failed")
	for {
		select {
		case message, ok := <-s.send:
			if !ok {
				return
			}

			s.ws.SetWriteDeadline(time.Now().Add(WriteWait))

			w, err := s.ws.NextWriter(websocket.TextMessage)

			if err != nil {
				return
			}

			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		}
	}
}

// ReadMessages reads messages from ws connection and send them to node
func (s *Session) ReadMessages() {
	// s.ws.SetReadLimit(MaxMessageSize)

	defer s.Disconnect("")

	for {
		_, message, err := s.ws.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				s.Log.Debugf("Websocket read error: %v", err)
			}
			break
		}

		s.node.HandleCommand(s, message)
	}
}

// Disconnect enqueues RPC disconnect request and closes the connection
func (s *Session) Disconnect(reason string) {
	s.mu.Lock()
	if !s.connected {
		// s.node.Disconnect(s)
	}
	s.connected = false
	s.mu.Unlock()

	s.Close(reason)
}

// Close websocket connection with the specified reason
func (s *Session) Close(reason string) {
	s.mu.Lock()
	if s.closed {
		return
	}
	s.closed = true
	s.mu.Unlock()

	// TODO: make deadline and status code configurable
	deadline := time.Now().Add(time.Second)
	msg := websocket.FormatCloseMessage(DefaultCloseStatus, reason)
	s.ws.WriteControl(websocket.CloseMessage, msg, deadline)
	s.ws.Close()
}

// UID returns session uid
func (s *Session) UID() string {
	return s.uid
}
