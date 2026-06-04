package main

import (
	"sync"
	"time"
)

type Room struct {
	id           string
	mu           sync.RWMutex
	clients      map[string]*Client
	lastActivity time.Time
}

func NewRoom(id string) *Room {
	return &Room{
		id:           id,
		clients:      make(map[string]*Client),
		lastActivity: time.Now(),
	}
}

func (r *Room) Add(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c.connID] = c
	r.lastActivity = time.Now()
}

func (r *Room) Remove(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c.connID)
	r.lastActivity = time.Now()
}

func (r *Room) ClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

func (r *Room) LastActivity() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastActivity
}

func (r *Room) Broadcast(senderConnID string, data []byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.lastActivity = time.Now()
	isVoice := isVoicePacket(data)
	for _, c := range r.clients {
		if c.connID == senderConnID || c.dead.Load() {
			continue
		}
		select {
		case c.send <- data:
		default:
			// Send buffer full.
			if isVoice {
				// Voice is real-time: dropping a frame is acceptable.
				continue
			}
			// Dropping a DATA frame would silently desync this peer.
			// Mark it dead and close the connection so it reconnects and
			// pulls a fresh snapshot. Never send to it again (avoids
			// send-on-closed-channel panics from later broadcasts).
			c.dead.Store(true)
			c.Close()
		}
	}
}

func (r *Room) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.clients {
		c.Close()
	}
}
