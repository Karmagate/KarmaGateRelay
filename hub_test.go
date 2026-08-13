package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func testConfig() *Config {
	return &Config{
		MaxRooms:          100,
		MaxClientsPerRoom: 10,
		MaxMessageSize:    1048576,
		RoomIdleTimeout:   1 * time.Hour,
		HostKeyGrace:      15 * time.Minute,
		RateLimitPerIP:    100,
	}
}

func TestHub_RegisterHostKey(t *testing.T) {
	hub := NewHub(testConfig())

	key := []byte("test-public-key-32-bytes-long!!")
	hub.RegisterHostKey("room-1", key)

	got := hub.GetHostKey("room-1")
	if got == nil {
		t.Fatal("expected host key, got nil")
	}
	if string(got) != string(key) {
		t.Errorf("got %q, want %q", got, key)
	}
}

func TestHub_GetHostKey_NotFound(t *testing.T) {
	hub := NewHub(testConfig())

	got := hub.GetHostKey("nonexistent")
	if got != nil {
		t.Errorf("expected nil for nonexistent room, got %v", got)
	}
}

func TestHub_RoomCount(t *testing.T) {
	hub := NewHub(testConfig())

	if hub.RoomCount() != 0 {
		t.Errorf("expected 0 rooms, got %d", hub.RoomCount())
	}

	hub.RegisterHostKey("room-1", []byte("key1"))
	// RoomCount tracks rooms with clients, not just host keys
	if hub.RoomCount() != 0 {
		t.Errorf("expected 0 rooms (no clients yet), got %d", hub.RoomCount())
	}
}

func TestHub_RunAndShutdown(t *testing.T) {
	hub := NewHub(testConfig())
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	cancel()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("hub.Run did not return after cancel")
	}
}

func TestHub_HostKeyGrace_AfterEmptyRoom(t *testing.T) {
	cfg := testConfig()
	cfg.HostKeyGrace = 200 * time.Millisecond
	hub := NewHub(cfg)

	key := []byte("test-public-key-32-bytes-long!!")
	hub.RegisterHostKey("room-grace", key)

	c := &Client{roomID: "room-grace", peerID: "p1", connID: "c1", send: make(chan []byte, 4)}
	hub.addClient(c)
	st := hub.RoomStatus("room-grace")
	if !st.Alive || st.Clients != 1 || !st.HasHostKey {
		t.Fatalf("status while live = %+v", st)
	}

	hub.removeClient(c)
	if hub.GetHostKey("room-grace") == nil {
		t.Fatal("host key should be retained during grace")
	}
	st = hub.RoomStatus("room-grace")
	if st.Alive || st.Clients != 0 || !st.HasHostKey {
		t.Fatalf("status during grace = %+v", st)
	}

	time.Sleep(250 * time.Millisecond)
	hub.cleanupExpiredHostKeys()
	if hub.GetHostKey("room-grace") != nil {
		t.Fatal("host key should be gone after grace")
	}
	st = hub.RoomStatus("room-grace")
	if st.HasHostKey {
		t.Fatalf("status after grace = %+v", st)
	}
}

func TestHub_HostKeyGrace_DisabledDropsImmediately(t *testing.T) {
	cfg := testConfig()
	cfg.HostKeyGrace = 0
	hub := NewHub(cfg)
	hub.RegisterHostKey("room-x", []byte("test-public-key-32-bytes-long!!"))
	c := &Client{roomID: "room-x", peerID: "p1", connID: "c1", send: make(chan []byte, 4)}
	hub.addClient(c)
	hub.removeClient(c)
	if hub.GetHostKey("room-x") != nil {
		t.Fatal("expected host key dropped immediately when grace=0")
	}
}

func TestHub_TryRegisterHostKey_SameKeyReclaim(t *testing.T) {
	hub := NewHub(testConfig())
	key := []byte("test-public-key-32-bytes-long!!")
	if err := hub.TryRegisterHostKey("room-r", key); err != nil {
		t.Fatal(err)
	}
	c := &Client{roomID: "room-r", peerID: "p1", connID: "c1", send: make(chan []byte, 4)}
	hub.addClient(c)
	hub.removeClient(c)
	// During grace, same key reclaim must succeed and clear expiry.
	if err := hub.TryRegisterHostKey("room-r", key); err != nil {
		t.Fatalf("same-key reclaim: %v", err)
	}
	if _, ok := hub.hostKeyExpiry["room-r"]; ok {
		t.Fatal("grace expiry should be cleared on same-key reclaim")
	}
}

func TestHub_TryRegisterHostKey_Conflict(t *testing.T) {
	hub := NewHub(testConfig())
	keyA := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	keyB := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err := hub.TryRegisterHostKey("room-c", keyA); err != nil {
		t.Fatal(err)
	}
	err := hub.TryRegisterHostKey("room-c", keyB)
	if !errors.Is(err, ErrHostKeyConflict) {
		t.Fatalf("want ErrHostKeyConflict, got %v", err)
	}
	got := hub.GetHostKey("room-c")
	if got == nil || string(got) != string(keyA) {
		t.Fatalf("original key must remain, got %q", got)
	}
}

func TestHub_TryRegisterHostKey_AfterGraceAllowsNewKey(t *testing.T) {
	cfg := testConfig()
	cfg.HostKeyGrace = 50 * time.Millisecond
	hub := NewHub(cfg)
	keyA := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	keyB := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	_ = hub.TryRegisterHostKey("room-g", keyA)
	c := &Client{roomID: "room-g", peerID: "p1", connID: "c1", send: make(chan []byte, 4)}
	hub.addClient(c)
	hub.removeClient(c)
	time.Sleep(80 * time.Millisecond)
	hub.cleanupExpiredHostKeys()
	if err := hub.TryRegisterHostKey("room-g", keyB); err != nil {
		t.Fatalf("after grace new host key should be allowed: %v", err)
	}
}
