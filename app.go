package main

import (
  "encoding/json"

  pb "./protos"
)

type App struct {
  Pinger *Pinger
  Subscriber *Subscriber
}

const (
  PING = "ping"
)

type Message struct {
  Command string `json:"command"`
  Identifier string `json:"identifier"`
  Data string `json:"data"`
}

type Reply struct {
  Type string `json:"type"`
  Identifier string `json:"identifier"`
  Message interface{} `json:"message"`
}

func (r *Reply) toJSON() []byte {
  jsonStr, err := json.Marshal(&r)
  if err != nil {
    panic("Failed to build JSON")
  }
  return jsonStr
}
  
var app = &App{}

func (app *App) Connected(conn *Conn, transmissions []string) {
  if hub.Size() == 0 {
    go app.Pinger.run()
  }
  hub.register <- conn

  Transmit(conn, transmissions)
}

func (app *App) Subscribe(conn *Conn, msg *Message) {
  if _, ok := conn.subscriptions[msg.Identifier]; ok {
    log.Warningf("Already Subscribed to %s", msg.Identifier)
    return
  }

  res := rpc.Subscribe(conn.identifiers, msg.Identifier)

  if res.Status == 1 {
    conn.subscriptions[msg.Identifier] = true
  }

  HandleReply(conn, msg, res)
}

func (app *App) Unsubscribe(conn *Conn, msg *Message) {
  if _, ok := conn.subscriptions[msg.Identifier]; !ok {
    log.Warningf("Unknown subscription %s", msg.Identifier)
    return
  }

  res := rpc.Unsubscribe(conn.identifiers, msg.Identifier)

  if res.Status == 1 {
    delete(conn.subscriptions, msg.Identifier)
  }

  HandleReply(conn, msg, res) 
}

func (app *App) Perform(conn *Conn, msg *Message) {
  if _, ok := conn.subscriptions[msg.Identifier]; !ok {
    log.Warningf("Unknown subscription %s", msg.Identifier)
    return
  }

  res := rpc.Perform(conn.identifiers, msg.Identifier, msg.Data)

  HandleReply(conn, msg, res) 
}

func (app *App) Disconnected(conn *Conn) {
  if hub.Size() == 1 {
    app.Pinger.pause()
  }

  hub.unregister <- conn

  rpc.Disconnect(conn.identifiers, SubscriptionsList(conn.subscriptions))
}

func (app *App) BroadcastAll(message []byte) {
  hub.broadcast <- message
}

func Transmit(conn *Conn, transmissions []string) {
  for _, msg := range transmissions {
    conn.send <- []byte(msg)
  }
}

func HandleReply(conn *Conn, msg *Message, reply *pb.CommandResponse) {
  if reply.Disconnect {
    defer conn.ws.Close()
  }

  if reply.StopStreams {
    hub.unsubscribe <- conn
  }

  if reply.StreamFrom {
    hub.subscribe <- &SubscriptionInfo{conn: conn, stream: reply.StreamId, identifier: msg.Identifier}
  }

  Transmit(conn, reply.Transmissions)
}

func SubscriptionsList(subs map[string]bool) []string {
  keys := []string{}
  for k := range subs {
    keys = append(keys, k)
  }
  return keys
}