package ws

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/anycable/anycable-go/version"
	"github.com/apex/log"
	"github.com/gorilla/websocket"
	nanoid "github.com/matoous/go-nanoid"
)

const remoteAddrHeader = "REMOTE_ADDR"

const ActionCableJSONProtocol = "actioncable-v1-json"
const ActionCableMsgpackProtocol = "actioncable-v1-msgpack"
const ActionCableProtobufProtocol = "actioncable-v1-protobuf"

var subprotocols = []string{ActionCableJSONProtocol, ActionCableMsgpackProtocol, ActionCableProtobufProtocol}

type RequestInfo struct {
	UID     string
	Url     string
	Headers *map[string]string
}

func NewRequestInfo(r *http.Request, headersToFetch []string) (*RequestInfo, error) {
	headers := FetchHeaders(r, headersToFetch)
	uid, err := FetchUID(r)

	if err != nil {
		return nil, errors.New("Failed to retrieve connection uid")
	}

	return &RequestInfo{UID: uid, Headers: &headers}, nil
}

type sessionHandler = func(conn *websocket.Conn, info *RequestInfo, callback func()) error

// WebsocketHandler generate a new http handler for WebSocket connections
func WebsocketHandler(headersToFetch []string, config *Config, sessionHandler sessionHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := log.WithField("context", "ws")

		upgrader := websocket.Upgrader{
			CheckOrigin:       CheckOrigin(config.AllowedOrigins),
			Subprotocols:      subprotocols,
			ReadBufferSize:    config.ReadBufferSize,
			WriteBufferSize:   config.WriteBufferSize,
			EnableCompression: config.EnableCompression,
		}

		rheader := map[string][]string{"X-AnyCable-Version": {version.Version()}}
		wsc, err := upgrader.Upgrade(w, r, rheader)
		if err != nil {
			ctx.Debugf("Websocket connection upgrade error: %#v", err.Error())
			return
		}

		url := r.URL.String()

		if !r.URL.IsAbs() {
			// See https://github.com/golang/go/issues/28940#issuecomment-441749380
			scheme := "http://"
			if r.TLS != nil {
				scheme = "https://"
			}
			url = fmt.Sprintf("%s%s%s", scheme, r.Host, url)
		}

		info, err := NewRequestInfo(r, headersToFetch)
		if err != nil {
			CloseWithReason(wsc, websocket.CloseAbnormalClosure, err.Error())
			return
		}
		info.Url = url

		wsc.SetReadLimit(config.MaxMessageSize)

		if config.EnableCompression {
			wsc.EnableWriteCompression(true)
		}

		sessionCtx := log.WithField("sid", info.UID)

		// Separate goroutine for better GC of caller's data.
		go func() {
			sessionCtx.Debugf("WebSocket session established")
			serr := sessionHandler(wsc, info, func() {
				sessionCtx.Debugf("WebSocket session completed")
			})

			if serr != nil {
				sessionCtx.Errorf("WebSocket session failed: %v", serr)
				return
			}
		}()
	})
}

// FetchHeaders extracts specified headers from request
func FetchHeaders(r *http.Request, list []string) map[string]string {
	res := make(map[string]string)

	for _, header := range list {
		res[header] = r.Header.Get(header)
	}
	res[remoteAddrHeader], _, _ = net.SplitHostPort(r.RemoteAddr)
	return res
}

// FetchUID safely extracts uid from `X-Request-ID` header or generates a new one
func FetchUID(r *http.Request) (string, error) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		return nanoid.Nanoid()
	}

	return requestID, nil
}

func CheckOrigin(origins string) func(r *http.Request) bool {
	if origins == "" {
		return func(r *http.Request) bool { return true }
	}

	hosts := strings.Split(strings.ToLower(origins), ",")

	return func(r *http.Request) bool {
		origin := strings.ToLower(r.Header.Get("Origin"))
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}

		for _, host := range hosts {
			if host[0] == '*' && strings.HasSuffix(u.Host, host[1:]) {
				return true
			}
			if u.Host == host {
				return true
			}
		}
		return false
	}
}
