package ws

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/anycable/anycable-go/version"
	"github.com/apex/log"
	"github.com/gorilla/websocket"
	nanoid "github.com/matoous/go-nanoid"
)

const remoteAddrHeader = "REMOTE_ADDR"

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

type sessionHandler = func(conn *websocket.Conn, info *RequestInfo) error

// WebsocketHandler generate a new http handler for WebSocket connections
func WebsocketHandler(headersToFetch []string, config *Config, sessionHandler sessionHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := log.WithField("context", "ws")

		upgrader := websocket.Upgrader{
			CheckOrigin:       func(r *http.Request) bool { return true },
			Subprotocols:      []string{"actioncable-v1-json"},
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
			serr := sessionHandler(wsc, info)

			if serr != nil {
				sessionCtx.Errorf("WebSocket session failed: %v", serr)
				return
			}

			sessionCtx.Debugf("WebSocket session completed")
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
