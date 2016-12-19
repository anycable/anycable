package main

import (
	"encoding/json"
)

type SubscriptionInfo struct {
	conn       *Conn
	stream     string
	identifier string
}

type StreamMessage struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

type Hub struct {
	// Registered connections.
	connections map[*Conn]bool

	// Messages for all connections.
	broadcast chan []byte

	// Messages for specified stream.
	stream_broadcast chan *StreamMessage

	// Register requests from the connections.
	register chan *Conn

	// Unregister requests from connections.
	unregister chan *Conn

	// Subscribe requests to strreams.
	subscribe chan *SubscriptionInfo

	// Unsubscribe requests from streams.
	unsubscribe chan *SubscriptionInfo

	// Maps streams to connections
	streams map[string]map[*Conn]string

	// Maps connections to identifiers to streams
	connection_streams map[*Conn]map[string][]string
}

var hub = Hub{
	broadcast:          make(chan []byte),
	stream_broadcast:   make(chan *StreamMessage),
	register:           make(chan *Conn),
	unregister:         make(chan *Conn),
	subscribe:          make(chan *SubscriptionInfo),
	unsubscribe:        make(chan *SubscriptionInfo),
	connections:        make(map[*Conn]bool),
	streams:            make(map[string]map[*Conn]string),
	connection_streams: make(map[*Conn]map[string][]string),
}

func (h *Hub) run() {
	for {
		select {
		case conn := <-h.register:
			log.Debugf("Register connection %v", conn)
			h.connections[conn] = true

		case conn := <-h.unregister:
			log.Debugf("Unregister connection %v", conn)

			h.UnsubscribeConnection(conn)

			if _, ok := h.connections[conn]; ok {
				delete(h.connections, conn)
				close(conn.send)
			}

		case message := <-h.broadcast:
			log.Debugf("Broadcast message %s", message)
			for conn := range h.connections {
				select {
				case conn.send <- message:
				default:
					close(conn.send)
					delete(hub.connections, conn)
				}
			}

		case stream_message := <-h.stream_broadcast:
			log.Debugf("Broadcast to stream %s: %s", stream_message.Stream, stream_message.Data)

			if _, ok := h.streams[stream_message.Stream]; !ok {
				log.Debugf("No connections for stream %s", stream_message.Stream)
				return
			}

			buf := make(map[string][]byte)

			for conn, id := range h.streams[stream_message.Stream] {
				var bdata []byte

				if msg, ok := buf[id]; ok {
					bdata = msg
				} else {
					bdata = BuildMessage(stream_message.Data, id)
					buf[id] = bdata
				}
				select {
				case conn.send <- bdata:
				default:
					close(conn.send)
					delete(hub.connections, conn)
				}
			}

		case subinfo := <-h.subscribe:
			log.Debugf("Subscribe to stream %s for %s", subinfo.stream, subinfo.conn.identifiers)

			if _, ok := h.streams[subinfo.stream]; !ok {
				h.streams[subinfo.stream] = make(map[*Conn]string)
			}

			h.streams[subinfo.stream][subinfo.conn] = subinfo.identifier

			if _, ok := h.connection_streams[subinfo.conn]; !ok {
				h.connection_streams[subinfo.conn] = make(map[string][]string)
			}

			h.connection_streams[subinfo.conn][subinfo.identifier] = append(
				h.connection_streams[subinfo.conn][subinfo.identifier],
				subinfo.stream)

		case subinfo := <-h.unsubscribe:
			h.UnsubscribeConnectionFromChannel(subinfo.conn, subinfo.identifier)
		}
	}
}

func (h *Hub) Size() int {
	return len(h.connections)
}

func (h *Hub) UnsubscribeConnection(conn *Conn) {
	log.Debugf("Unsubscribe from all streams: %s", conn.identifiers)

	for channel, _ := range h.connection_streams[conn] {
		h.UnsubscribeConnectionFromChannel(conn, channel)
	}

	delete(h.connection_streams, conn)
}

func (h *Hub) UnsubscribeConnectionFromChannel(conn *Conn, channel string) {
	log.Debugf("Unsubscribe from channel %s: %s", channel, conn.identifiers)

	if _, ok := h.connection_streams[conn]; !ok {
		return
	}

	for _, stream := range h.connection_streams[conn][channel] {
		delete(h.streams[stream], conn)

		if len(h.streams[stream]) == 0 {
			delete(h.streams, stream)
		}
	}
}

func BuildMessage(data string, identifier string) []byte {
	var msg map[string]interface{}

	json.Unmarshal([]byte(data), &msg)

	return (&Reply{Identifier: identifier, Message: msg}).toJSON()
}
