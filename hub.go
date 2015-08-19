package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"time"
)

const (
	WRITE_WAIT  = 10 * time.Second
	PONG_WAIT   = 60 * time.Second
	PING_PERIOD = (PONG_WAIT * 9) / 10
)

type Connection struct {
	Socket *websocket.Conn
	Send   chan string
}

func (self *Connection) readPump() {
	defer func() {
		hub.Unregister <- self
		self.Socket.Close()
	}()

	self.Socket.SetReadDeadline(time.Now().Add(PONG_WAIT))
	self.Socket.SetPongHandler(func(string) error {
		self.Socket.SetReadDeadline(time.Now().Add(PONG_WAIT))
		return nil
	})

	for {
		if _, message, err := self.Socket.ReadMessage(); err == nil {
			self.write(websocket.TextMessage, message)
		} else {
			break
		}
	}
}

func (self *Connection) write(messageType int, data []byte) error {
	self.Socket.SetWriteDeadline(time.Now().Add(WRITE_WAIT))
	return self.Socket.WriteMessage(messageType, data)
}

func (self *Connection) writePump() {
	ticker := time.NewTicker(PING_PERIOD)

	defer ticker.Stop()
	defer self.Socket.Close()

	for {
		select {
		case message, ok := <-self.Send:
			if !ok {
				self.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := self.write(websocket.TextMessage, []byte(message)); err != nil {
				return
			}
		case <-ticker.C:
			if err := self.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func NewConnection(socket *websocket.Conn) *Connection {
	return &Connection{
		Send:   make(chan string, 128),
		Socket: socket,
	}
}

type ConnectionsHub struct {
	Connections map[*Connection]bool
	Send        chan string
	Register    chan *Connection
	Unregister  chan *Connection
}

func (self *ConnectionsHub) run() {
	for {
		select {
		case c := <-self.Register:
			self.Connections[c] = true
		case c := <-self.Unregister:
			if _, ok := self.Connections[c]; ok {
				delete(self.Connections, c)
				close(c.Send)
			}
		case m := <-self.Send:
			log.WithField("data", m).Debug("Send")
			for conn, _ := range self.Connections {
				conn.Send <- m
			}
		}
	}
}
