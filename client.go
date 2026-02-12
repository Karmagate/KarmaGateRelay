package main

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	sendBufferSize = 256
)

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	roomID string
	peerID string
	role   string
	ip     string
	send   chan []byte

	closeOnce sync.Once
}

func NewClient(hub *Hub, conn *websocket.Conn, roomID, peerID, role, ip string) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		roomID: roomID,
		peerID: peerID,
		role:   role,
		ip:     ip,
		send:   make(chan []byte, sendBufferSize),
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("read error peer=%s room=%s: %v", c.peerID, c.roomID, err)
			}
			return
		}

		c.hub.Broadcast(&BroadcastMsg{
			RoomID:   c.roomID,
			SenderID: c.peerID,
			Data:     message,
		})
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Drain queued messages into the same write
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.send)
	})
}
