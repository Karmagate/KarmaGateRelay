package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// ErrHostKeyConflict means a different Ed25519 host pubkey tried to claim a
// room that already has a non-expired host key (hijack / wrong reclaim).
var ErrHostKeyConflict = errors.New("host key conflict")

type Hub struct {
	cfg *Config

	mu       sync.RWMutex
	rooms    map[string]*Room
	hostKeys map[string][]byte // room_id → host Ed25519 public key
	// hostKeyExpiry tracks when a grace-kept host key should be dropped
	// after the room became empty. Zero time = key is active with live clients
	// (or freshly registered) and must not expire via grace.
	hostKeyExpiry map[string]time.Time

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
		cfg:           cfg,
		rooms:         make(map[string]*Room),
		hostKeys:      make(map[string][]byte),
		hostKeyExpiry: make(map[string]time.Time),
		registerCh:    make(chan *Client, 64),
		unregisterCh:  make(chan *Client, 64),
		broadcastCh:   make(chan *BroadcastMsg, 2048),
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
			h.cleanupExpiredHostKeys()
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

// RegisterHostKey stores a host pubkey (tests / trusted paths). Prefer
// TryRegisterHostKey on the WS accept path so a different key cannot steal a room.
func (h *Hub) RegisterHostKey(roomID string, pubKey []byte) {
	_ = h.TryRegisterHostKey(roomID, pubKey)
}

// TryRegisterHostKey registers or refreshes the host pubkey for roomID.
//
// Rules (safe reclaim model):
//   - No key / expired grace key → accept (fresh create or post-grace reclaim).
//   - Same key as retained → accept, clear grace (creator resume / reconnect).
//   - Different non-expired key → ErrHostKeyConflict (no overwrite).
func (h *Hub) TryRegisterHostKey(roomID string, pubKey []byte) error {
	if roomID == "" || len(pubKey) == 0 {
		return fmt.Errorf("invalid room or pubkey")
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, ok := h.hostKeys[roomID]; ok && len(existing) > 0 {
		expired := false
		if exp, ok := h.hostKeyExpiry[roomID]; ok && !exp.IsZero() && time.Now().After(exp) {
			expired = true
		}
		if !expired {
			if len(existing) != len(pubKey) || subtle.ConstantTimeCompare(existing, pubKey) != 1 {
				return ErrHostKeyConflict
			}
			// Same creator key: clear grace so guests can join again.
			delete(h.hostKeyExpiry, roomID)
			return nil
		}
		// Grace elapsed — drop stale key before accepting reclaim/create.
		delete(h.hostKeys, roomID)
		delete(h.hostKeyExpiry, roomID)
	}

	key := make([]byte, len(pubKey))
	copy(key, pubKey)
	h.hostKeys[roomID] = key
	delete(h.hostKeyExpiry, roomID)
	return nil
}

func (h *Hub) GetHostKey(roomID string) []byte {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if exp, ok := h.hostKeyExpiry[roomID]; ok && !exp.IsZero() && time.Now().After(exp) {
		return nil
	}
	return h.hostKeys[roomID]
}

// HasHostKey reports whether a non-expired host key is retained for roomID.
func (h *Hub) HasHostKey(roomID string) bool {
	return h.GetHostKey(roomID) != nil
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

// RoomStatus is the public probe payload for Resume UI (no secrets).
type RoomStatus struct {
	Alive      bool `json:"alive"`
	Clients    int  `json:"clients"`
	HasHostKey bool `json:"has_host_key"`
}

func (h *Hub) RoomStatus(roomID string) RoomStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients := 0
	if room, ok := h.rooms[roomID]; ok {
		clients = room.ClientCount()
	}
	hasKey := false
	if key, ok := h.hostKeys[roomID]; ok && len(key) > 0 {
		if exp, ok := h.hostKeyExpiry[roomID]; ok && !exp.IsZero() && time.Now().After(exp) {
			hasKey = false
		} else {
			hasKey = true
		}
	}
	return RoomStatus{
		Alive:      clients > 0,
		Clients:    clients,
		HasHostKey: hasKey,
	}
}

func (h *Hub) addClient(c *Client) {
	h.mu.Lock()
	room, ok := h.rooms[c.roomID]
	if !ok {
		room = NewRoom(c.roomID)
		h.rooms[c.roomID] = room
	}
	// A live client cancels host-key grace for this room.
	delete(h.hostKeyExpiry, c.roomID)
	h.mu.Unlock()

	room.Add(c)
	connShort := c.connID
	if len(connShort) > 8 {
		connShort = connShort[:8]
	}
	log.Printf("peer %s (conn=%s) joined room %s (role=%s)", c.peerID, connShort, c.roomID, c.role)

	if c.conn != nil {
		go c.ReadPump()
		go c.WritePump()
	}
}

func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	room, ok := h.rooms[c.roomID]
	if ok {
		room.Remove(c)
		if room.ClientCount() == 0 {
			delete(h.rooms, c.roomID)
			// Keep hostKeys for HostKeyGrace so guests/creator resume can still
			// authenticate briefly after the room empties.
			grace := h.cfg.HostKeyGrace
			if grace <= 0 {
				delete(h.hostKeys, c.roomID)
				delete(h.hostKeyExpiry, c.roomID)
				log.Printf("room %s destroyed (no clients, no host-key grace)", c.roomID)
			} else {
				h.hostKeyExpiry[c.roomID] = time.Now().Add(grace)
				log.Printf("room %s empty; host key retained for %v", c.roomID, grace)
			}
		} else {
			// Notify remaining peers that this client disconnected.
			// Generate a synthetic session:leave envelope so clients
			// can remove the peer from their room.
			// NOTE: this skeleton is plaintext routing metadata only (peer id).
			// Clients must NOT treat Creator disconnect as End Session — use host_away.
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
			delete(h.hostKeyExpiry, id)
			log.Printf("room %s cleaned up (idle timeout)", id)
		}
	}
}

func (h *Hub) cleanupExpiredHostKeys() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for id, exp := range h.hostKeyExpiry {
		if exp.IsZero() || now.Before(exp) {
			continue
		}
		// Only drop if room is still empty.
		if room, ok := h.rooms[id]; ok && room.ClientCount() > 0 {
			delete(h.hostKeyExpiry, id)
			continue
		}
		delete(h.hostKeys, id)
		delete(h.hostKeyExpiry, id)
		log.Printf("host key for room %s expired (grace elapsed)", id)
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
	h.hostKeyExpiry = make(map[string]time.Time)
}
