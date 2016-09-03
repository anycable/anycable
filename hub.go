package main

import (
  "encoding/json"
)

type SubscriptionInfo struct {
  conn *Conn
  stream string
  identifier string
}

type StreamMessage struct {
  Stream string `json:"stream"`
  Data string `json:"data"`
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
  unsubscribe chan *Conn

  // Maps streams to connections
  streams map[string]map[*Conn]bool

  // Maps connections to streams
  connection_streams map[*Conn][]string

  // Maps streams to channels
  stream_channel map[string]string
}

var hub = Hub{
  broadcast: make(chan []byte),
  stream_broadcast: make(chan *StreamMessage),
  register: make(chan *Conn),
  unregister: make(chan *Conn),
  subscribe: make(chan *SubscriptionInfo),
  unsubscribe: make(chan *Conn),
  connections: make(map[*Conn]bool),
  streams: make(map[string]map[*Conn]bool),
  connection_streams: make(map[*Conn][]string),
  stream_channel: make(map[string]string),
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

      case stream_message := <- h.stream_broadcast:
        log.Debugf("Broadcast to stream %s: %s", stream_message.Stream, stream_message.Data)

        if _, ok := h.streams[stream_message.Stream]; !ok {
          log.Debugf("No connections for stream %s", stream_message.Stream)
          return
        }

        identifier := h.stream_channel[stream_message.Stream]
        
        var msg map[string]interface{}

        json.Unmarshal([]byte(stream_message.Data), &msg)

        bdata := (&Reply{Identifier: identifier, Message: msg}).toJSON()

        for conn := range h.streams[stream_message.Stream] {
          select {
            case conn.send <- bdata:
            default:
              close(conn.send)
              delete(hub.connections, conn)
          }
        }

      case subinfo := <- h.subscribe:
        log.Debugf("Subscribe to stream %s for %s", subinfo.stream, subinfo.conn.identifiers)

        if _, ok := h.streams[subinfo.stream]; !ok {
          h.streams[subinfo.stream] = make(map[*Conn]bool)
        }
        
        h.stream_channel[subinfo.stream] = subinfo.identifier

        h.streams[subinfo.stream][subinfo.conn] = true

        h.connection_streams[subinfo.conn] = append(
          h.connection_streams[subinfo.conn],
          subinfo.stream)

      case conn := <- h.unsubscribe:
        h.UnsubscribeConnection(conn)
    }
  }
}

func (h *Hub) Size() int {
  return len(h.connections)
}

func (h *Hub) UnsubscribeConnection(conn *Conn) {
  log.Debugf("Unsubscribe from all streams %s", conn.identifiers)

  for _, stream := range h.connection_streams[conn] {
    delete(h.streams[stream], conn)

    if len(h.streams[stream]) == 0 {
      delete(h.streams, stream)
      delete(h.stream_channel, stream)
    }
  }

  delete(h.connection_streams, conn)
}
