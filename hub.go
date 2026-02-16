package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type Hub struct {
	cfg *Config

	mu       sync.RWMutex
	rooms    map[string]*Room
	hostKeys map[string][]byte // room_id → host Ed25519 public key

	registerCh   chan *Client
	unregisterCh chan *Client
	broadcastCh  chan *BroadcastMsg
}

type BroadcastMsg struct {
	RoomID   string
	SenderID string
	Data     []byte
}

func NewHub(cfg *Config) *Hub {
	return &Hub{
		cfg:          cfg,
		rooms:        make(map[string]*Room),
		hostKeys:     make(map[string][]byte),
		registerCh:   make(chan *Client, 64),
		unregisterCh: make(chan *Client, 64),
		broadcastCh:  make(chan *BroadcastMsg, 2048),
	}
}

func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.closeAll()
			return

		case client := <-h.registerCh:
			h.addClient(client)

		case client := <-h.unregisterCh:
			h.removeClient(client)

		case msg := <-h.broadcastCh:
			h.broadcast(msg)

		case <-ticker.C:
			h.cleanupIdleRooms()
		}
	}
}

func (h *Hub) Register(c *Client) {
	h.registerCh <- c
}

func (h *Hub) Unregister(c *Client) {
	h.unregisterCh <- c
}

func (h *Hub) Broadcast(msg *BroadcastMsg) {
	h.broadcastCh <- msg
}

func (h *Hub) RegisterHostKey(roomID string, pubKey []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := make([]byte, len(pubKey))
	copy(key, pubKey)
	h.hostKeys[roomID] = key
}

func (h *Hub) GetHostKey(roomID string) []byte {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.hostKeys[roomID]
}

func (h *Hub) RoomCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

func (h *Hub) ClientCount(roomID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if room, ok := h.rooms[roomID]; ok {
		return room.ClientCount()
	}
	return 0
}

func (h *Hub) addClient(c *Client) {
	h.mu.Lock()
	room, ok := h.rooms[c.roomID]
	if !ok {
		room = NewRoom(c.roomID)
		h.rooms[c.roomID] = room
	}
	h.mu.Unlock()

	room.Add(c)
	log.Printf("peer %s (conn=%s) joined room %s (role=%s)", c.peerID, c.connID[:8], c.roomID, c.role)

	go c.ReadPump()
	go c.WritePump()
}

func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	room, ok := h.rooms[c.roomID]
	if ok {
		room.Remove(c)
		if room.ClientCount() == 0 {
			delete(h.rooms, c.roomID)
			delete(h.hostKeys, c.roomID)
			log.Printf("room %s destroyed (no clients)", c.roomID)
		} else {
			// Notify remaining peers that this client disconnected.
			// Generate a synthetic session:leave envelope so clients
			// can remove the peer from their room.
			notification := []byte(fmt.Sprintf(
				`{"id":"","type":"session:leave","from":"%s","ts":%d,"nonce":0,"payload":null,"sig":null}`,
				c.peerID, time.Now().UnixMilli(),
			))
			room.Broadcast(c.connID, notification)
		}
	}
	h.mu.Unlock()

	log.Printf("peer %s left room %s", c.peerID, c.roomID)
}

func (h *Hub) broadcast(msg *BroadcastMsg) {
	h.mu.RLock()
	room, ok := h.rooms[msg.RoomID]
	h.mu.RUnlock()

	if !ok {
		return
	}

	room.Broadcast(msg.SenderID, msg.Data)
}

func (h *Hub) cleanupIdleRooms() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for id, room := range h.rooms {
		if now.Sub(room.LastActivity()) > h.cfg.RoomIdleTimeout {
			room.CloseAll()
			delete(h.rooms, id)
			delete(h.hostKeys, id)
			log.Printf("room %s cleaned up (idle timeout)", id)
		}
	}
}

func (h *Hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, room := range h.rooms {
		room.CloseAll()
	}
	h.rooms = make(map[string]*Room)
	h.hostKeys = make(map[string][]byte)
}
