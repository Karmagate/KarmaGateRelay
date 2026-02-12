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
	r.clients[c.peerID] = c
	r.lastActivity = time.Now()
}

func (r *Room) Remove(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c.peerID)
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

func (r *Room) Broadcast(senderID string, data []byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.lastActivity = time.Now()
	for _, c := range r.clients {
		if c.peerID == senderID {
			continue
		}
		select {
		case c.send <- data:
		default:
			// Client's send buffer full — drop message
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
