package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/namsral/flag"
	"github.com/op/go-logging"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = 3 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// Conn is an middleman between the websocket connection and the hub.
type Conn struct {
	// The websocket connection.
	ws *websocket.Conn

	// Request path
	path string

	// Selected request headers
	headers map[string]string

	// Connection identifiers as received from RPC server
	identifiers string

	// Connection subscriptions
	subscriptions map[string]bool

	// Buffered channel of outbound messages.
	send chan []byte
}

type Config struct {
	// List of headers to proxy to RPC
	headers []string
}

var (
	config = &Config{}

	version = "0.5.2"

	log = logging.MustGetLogger("main")

	rpchost        = flag.String("rpc", "0.0.0.0:50051", "rpc service address")
	redishost      = flag.String("redis", "redis://localhost:6379/5", "redis address")
	redischannel   = flag.String("redis_channel", "__anycable__", "redis channel")
	addr           = flag.String("addr", "localhost:8080", "http service address")
	wspath         = flag.String("wspath", "/cable", "WS endpoint path")
	disconnectRate = flag.Int("disconnect_rate", 100, "the number of Disconnect calls per second")
	headers_list   = flag.String("headers", "cookie", "list of headers to proxy to RPC")
	sslCert        = flag.String("ssl_cert", "", "SSL certificate path")
	sslKey         = flag.String("ssl_key", "", "SSL private key path")

	upgrader = websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		Subprotocols:    []string{"actioncable-v1-json"},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// readPump pumps messages from the websocket connection to the hub.
func (c *Conn) readPump() {
	defer func() {
		log.Debugf("Disconnect on read error")
		app.Disconnected(c)
		CloseWS(c.ws, "Read Failed")
	}()
	for {
		_, message, err := c.ws.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Debugf("read error: %v", err)
			}
			break
		}

		msg := &Message{}

		if err := json.Unmarshal(message, &msg); err != nil {
			log.Debugf("Unknown message: %s", message)
		} else {
			log.Debugf("Client message: %s", msg)
			switch msg.Command {
			case "subscribe":
				app.Subscribe(c, msg)
			case "unsubscribe":
				app.Unsubscribe(c, msg)
			case "message":
				app.Perform(c, msg)
			default:
				log.Debugf("Unknown command: %s", msg.Command)
			}
		}
	}
}

// write writes a message with the given message type and payload.
func (c *Conn) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

func (c *Conn) writePump() {
	defer CloseWS(c.ws, "Write Failed")
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// The hub closed the channel.
				c.write(websocket.CloseMessage, []byte{})
				return
			}

			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			w, err := c.ws.NextWriter(websocket.TextMessage)
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

// serveWs handles websocket requests from the peer.
func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Critical(err)
		return
	}

	path := r.URL.String()
	headers := GetHeaders(r, config.headers)

	response, err := rpc.VerifyConnection(path, headers)

	if err != nil {
		log.Errorf("RPC Connect Error: %v", err)
		CloseWS(ws, "RPC Error")
		return
	}

	log.Debugf("Auth %s", response)

	status := response.Status.String()

	if status == "ERROR" {
		log.Errorf("Application error: %s", response.ErrorMsg)
		CloseWS(ws, "Application Error")
		return
	}

	if status == "FAILURE" {
		log.Warningf("Unauthenticated")
		CloseWS(ws, "Unauthenticated")
		return
	}

	conn := &Conn{send: make(chan []byte, 256), ws: ws, path: path, headers: headers, identifiers: response.Identifiers, subscriptions: make(map[string]bool)}
	app.Connected(conn, response.Transmissions)
	go conn.writePump()
	conn.readPump()
}

func CloseWS(ws *websocket.Conn, reason string) {
	deadline := time.Now().Add(time.Second)
	msg := websocket.FormatCloseMessage(3000, reason)
	ws.WriteControl(websocket.CloseMessage, msg, deadline)
	ws.Close()
}

func GetHeaders(r *http.Request, list []string) map[string]string {
	res := make(map[string]string)

	for _, header := range list {
		res[header] = r.Header.Get(header)
	}
	return res
}

func ParseHeadersArg(str string) []string {
	parts := strings.Split(str, ",")

	res := make([]string, len(parts))

	for i, v := range parts {
		res[i] = strings.ToLower(v)
	}
	return res
}

func main() {
	logflag := flag.Bool("log", false, "enable verbose logging")
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()

	config.headers = ParseHeadersArg(*headers_list)

	if *showVersion {
		fmt.Println(version)
		return
	}

	backend := logging.AddModuleLevel(logging.NewLogBackend(os.Stderr, "", 0))

	if *logflag {
		backend.SetLevel(logging.DEBUG, "")
	} else {
		backend.SetLevel(logging.INFO, "")
	}

	logging.SetBackend(backend)

	go hub.run()

	app.Pinger = NewPinger(pingPeriod)
	go app.Pinger.run()

	rpc.Init(*rpchost)
	defer rpc.Close()

	app.Subscriber = &Subscriber{host: *redishost, channel: *redischannel}
	go app.Subscriber.run()

	app.Disconnector = &DisconnectNotifier{rate: *disconnectRate, disconnect: make(chan *Conn)}
	go app.Disconnector.run()

	http.HandleFunc(*wspath, serveWs)

	if (*sslCert != "") && (*sslKey != "") {
		log.Infof("Running AnyCable websocket server (secured) v%s on %s at %s", version, *addr, *wspath)

		err := http.ListenAndServeTLS(*addr, *sslCert, *sslKey, nil)
		if err != nil {
			log.Fatal("HTTPS Server Error: ", err)
		}
	} else {
		log.Infof("Running AnyCable websocket server v%s on %s at %s", version, *addr, *wspath)

		err := http.ListenAndServe(*addr, nil)
		if err != nil {
			log.Fatal("HTTP Server Error: ", err)
		}
	}
}
