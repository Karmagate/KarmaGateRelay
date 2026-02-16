package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 60 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	sendBufferSize = 512

	// voiceMagic0/1 are the magic bytes that identify voice packets (0x4B56 = "KV").
	// Voice packets must be sent as individual WebSocket frames — never batched
	// with newline separators — because encrypted binary data may contain 0x0A bytes.
	voiceMagic0 = 0x4B
	voiceMagic1 = 0x56
)

func newConnID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// extractFromField extracts the "from" field from a JSON envelope.
// Returns empty string on any error.
func extractFromField(data []byte) string {
	var env struct {
		From string `json:"from"`
	}
	if json.Unmarshal(data, &env) == nil {
		return env.From
	}
	return ""
}

// isVoicePacket returns true if data starts with the voice magic bytes.
func isVoicePacket(data []byte) bool {
	return len(data) >= 2 && data[0] == voiceMagic0 && data[1] == voiceMagic1
}

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	roomID string
	peerID string // from JWT (used in leave notifications)
	connID string // unique per connection (used for room tracking)
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
		connID: newConnID(),
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

	peerIDLearned := false
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("read error peer=%s room=%s: %v", c.peerID, c.roomID, err)
			}
			return
		}

		// Learn the client's actual peerID from the first non-voice message.
		// The client may generate a fresh UUID that differs from the JWT's
		// peer_id (e.g. when multiple guests reuse one invite link).
		if !peerIDLearned && !isVoicePacket(message) {
			if realID := extractFromField(message); realID != "" && realID != c.peerID {
				log.Printf("peer %s identified as %s (room %s)", c.peerID, realID, c.roomID)
				c.peerID = realID
			}
			peerIDLearned = true
		}

		c.hub.Broadcast(&BroadcastMsg{
			RoomID:   c.roomID,
			SenderID: c.connID,
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

			// Voice packets MUST be sent as individual frames — never batched.
			// Encrypted binary voice data may contain 0x0A (newline) bytes,
			// which would corrupt the message if batched with '\n' separator.
			if isVoicePacket(message) {
				if err := c.conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
					return
				}
				// After sending voice, drain any more voice packets immediately
				// for minimal latency, but don't batch them.
				for {
					select {
					case next, ok2 := <-c.send:
						if !ok2 {
							return
						}
						if isVoicePacket(next) {
							if err := c.conn.WriteMessage(websocket.BinaryMessage, next); err != nil {
								return
							}
						} else {
							// Non-voice message: send it normally, potentially batching
							if err := c.writeDataMessage(next); err != nil {
								return
							}
							goto afterDrain
						}
					default:
						goto afterDrain
					}
				}
			afterDrain:
				continue
			}

			// Data (non-voice) messages: batch with newline separator for throughput
			if err := c.writeDataMessage(message); err != nil {
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

// writeDataMessage writes a data message, batching any queued non-voice messages.
func (c *Client) writeDataMessage(message []byte) error {
	w, err := c.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return err
	}
	_, _ = w.Write(message)

	// Drain queued data messages into the same write (batching for throughput).
	// Voice packets in the queue are sent separately after closing this writer.
	var pendingVoice [][]byte
	n := len(c.send)
	for i := 0; i < n; i++ {
		next := <-c.send
		if isVoicePacket(next) {
			pendingVoice = append(pendingVoice, next)
		} else {
			_, _ = w.Write([]byte{'\n'})
			_, _ = w.Write(next)
		}
	}

	if err := w.Close(); err != nil {
		return err
	}

	// Flush any voice packets that were queued between data messages
	for _, vp := range pendingVoice {
		if err := c.conn.WriteMessage(websocket.BinaryMessage, vp); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.send)
	})
}
