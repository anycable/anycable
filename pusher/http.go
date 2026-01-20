package pusher

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/anycable/anycable-go/broadcast"
	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/utils"
)

type RestAPI struct {
	conf     *Config
	verifier *Verifier
	log      *slog.Logger
	handler  broadcast.Handler
	broker   broker.Broker

	enableCORS   bool
	allowedHosts []string
}

var channelUsersPathPattern = regexp.MustCompile(`/channels/([^/]+)/users$`)

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

func NewRestAPI(h broadcast.Handler, b broker.Broker, c *Config, l *slog.Logger) *RestAPI {
	verifier := NewVerifier(c.AppKey, c.Secret)
	api := &RestAPI{handler: h, broker: b, verifier: verifier, conf: c, log: l}

	if api.conf.AddCORSHeaders {
		api.enableCORS = true
		if api.conf.CORSHosts != "" {
			api.allowedHosts = strings.Split(api.conf.CORSHosts, ",")
		} else {
			api.allowedHosts = []string{}
		}
	}

	return api
}

func (api *RestAPI) Handler(w http.ResponseWriter, r *http.Request) {
	if api.enableCORS {
		server.WriteCORSHeaders(w, r, api.allowedHosts)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// See https://pusher.com/docs/channels/library_auth_reference/rest-api/#worked-authentication-example
	path := r.URL.Path
	queryParams := r.URL.Query()
	key := queryParams.Get("auth_key")
	authTimestamp := queryParams.Get("auth_timestamp")
	authVersion := queryParams.Get("auth_version")
	signature := queryParams.Get("auth_signature")

	var stringToSign string

	switch r.Method {
	case http.MethodPost:
		bodyMD5 := queryParams.Get("body_md5")
		stringToSign = "POST\n" +
			path + "\n" +
			"auth_key=" + key +
			"&auth_timestamp=" + authTimestamp +
			"&auth_version=" + authVersion +
			"&body_md5=" + bodyMD5
	case http.MethodGet:
		stringToSign = "GET\n" +
			path + "\n" +
			"auth_key=" + key +
			"&auth_timestamp=" + authTimestamp +
			"&auth_version=" + authVersion
	default:
		api.log.Debug("invalid request method", "method", r.Method)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	if !api.verifier.Verify(stringToSign, signature) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodPost:
		api.handleEvents(w, r)
	case http.MethodGet:
		api.handleGetUsers(w, r)
	}
}

func (api *RestAPI) handleEvents(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)

	if err != nil {
		api.log.Error("failed to read request body")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	var pusherEvent *PusherEvent

	err = json.Unmarshal(body, &pusherEvent)

	if err != nil {
		api.log.Error("failed to parse Pusher event")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	var channels []string

	if pusherEvent.Channel != "" {
		channels = append(channels, pusherEvent.Channel)
	} else {
		channels = append(channels, pusherEvent.Channels...)
	}

	api.log.Debug("event received", "event", pusherEvent.Name, "channels", channels, "info", pusherEvent.Info)

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

		api.handler.HandleBroadcast(utils.ToJSON(msg)) // nolint:errcheck
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}")) // nolint:errcheck
}

func (api *RestAPI) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	matches := channelUsersPathPattern.FindStringSubmatch(r.URL.Path)

	if len(matches) < 2 {
		api.log.Debug("invalid path for get users", "path", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	channel := matches[1]

	if !strings.HasPrefix(channel, "presence-") {
		api.log.Debug("get users is only supported for presence channels", "channel", channel)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	info, err := api.broker.PresenceInfo(channel)

	if err != nil {
		api.log.Debug("failed to get presence info", "channel", channel, "error", err)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"users":[]}`)) // nolint:errcheck
		return
	}

	users := make([]map[string]string, len(info.Records))
	for i, record := range info.Records {
		users[i] = map[string]string{"id": record.ID}
	}

	response := map[string]interface{}{"users": users}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(utils.ToJSON(response)) // nolint:errcheck
}
