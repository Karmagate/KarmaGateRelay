package main

import (
	"context"
	"testing"
	"time"
)

func testConfig() *Config {
	return &Config{
		MaxRooms:          100,
		MaxClientsPerRoom: 10,
		MaxMessageSize:    1048576,
		RoomIdleTimeout:   1 * time.Hour,
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
