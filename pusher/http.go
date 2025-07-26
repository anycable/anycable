package pusher

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/anycable/anycable-go/broadcast"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/utils"
)

type Broadcaster struct {
	conf     *Config
	verifier *Verifier
	log      *slog.Logger
	handler  broadcast.Handler

	enableCORS   bool
	allowedHosts []string
}

type PusherEvent struct {
	Name     string   `json:"name"`
	Data     string   `json:"data"`
	Channels []string `json:"channels,omitempty"`
	Channel  string   `json:"channel,omitempty"`
	SocketID string   `json:"socket_id,omitempty"`
	Info     string   `json:"info,omitempty"`
}

type PusherBroadcast struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

func NewBroadcaster(h broadcast.Handler, c *Config, l *slog.Logger) *Broadcaster {
	verifier := NewVerifier(c.AppKey, c.Secret)
	b := &Broadcaster{handler: h, verifier: verifier, conf: c, log: l}

	if b.conf.AddCORSHeaders {
		b.enableCORS = true
		if b.conf.CORSHosts != "" {
			b.allowedHosts = strings.Split(b.conf.CORSHosts, ",")
		} else {
			b.allowedHosts = []string{}
		}
	}

	return b
}

func (b *Broadcaster) Handler(w http.ResponseWriter, r *http.Request) {
	if b.enableCORS {
		// Write CORS headers
		server.WriteCORSHeaders(w, r, b.allowedHosts)

		// Respond to preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	if r.Method != "POST" {
		b.log.Debug("invalid request method", "method", r.Method)
		w.WriteHeader(422)
		return
	}

	// See https://pusher.com/docs/channels/library_auth_reference/rest-api/#worked-authentication-example
	path := r.URL.Path
	queryParams := r.URL.Query()
	key := queryParams.Get("auth_key")
	authTimestamp := queryParams.Get("auth_timestamp")
	authVersion := queryParams.Get("auth_version")
	bodyMD5 := queryParams.Get("body_md5")
	signature := queryParams.Get("auth_signature")

	// Build the string to sign
	stringToSign := "POST\n" +
		path + "\n" +
		"auth_key=" + key +
		"&auth_timestamp=" + authTimestamp +
		"&auth_version=" + authVersion +
		"&body_md5=" + bodyMD5

	if !b.verifier.Verify(stringToSign, signature) {
		w.WriteHeader(401)
		return
	}

	body, err := io.ReadAll(r.Body)

	if err != nil {
		b.log.Error("failed to read request body")
		w.WriteHeader(422)
		return
	}

	var pusherEvent *PusherEvent

	err = json.Unmarshal(body, &pusherEvent)

	if err != nil {
		b.log.Error("failed to parse Pusher event")
		w.WriteHeader(422)
		return
	}

	var channels []string

	if pusherEvent.Channel != "" {
		channels = append(channels, pusherEvent.Channel)
	} else {
		channels = append(channels, pusherEvent.Channels...)
	}

	b.log.Debug("event received", "event", pusherEvent.Name, "channels", channels, "info", pusherEvent.Info)

	for _, channel := range channels {
		msg := common.StreamMessage{
			Stream: channel,
			Data: string(utils.ToJSON(
				PusherBroadcast{
					Event: pusherEvent.Name,
					Data:  pusherEvent.Data,
				})),
		}

		if pusherEvent.SocketID != "" {
			msg.Meta = &common.StreamMessageMetadata{
				ExcludeSocket: pusherEvent.SocketID,
			}
		}

		b.handler.HandleBroadcast(utils.ToJSON(msg))
	}

	// Respond with an empty JSON hash; currently, Info is not supported
	w.WriteHeader(200)
	w.Write([]byte("{}")) // nolint: errcheck
}
