package main

import (
	"testing"
	"time"
)

func TestRoom_AddRemove(t *testing.T) {
	room := NewRoom("test-room")

	c1 := &Client{peerID: "peer-1", connID: "conn-1", send: make(chan []byte, 10)}
	c2 := &Client{peerID: "peer-2", connID: "conn-2", send: make(chan []byte, 10)}

	room.Add(c1)
	if room.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", room.ClientCount())
	}

	room.Add(c2)
	if room.ClientCount() != 2 {
		t.Errorf("expected 2 clients, got %d", room.ClientCount())
	}

	room.Remove(c1)
	if room.ClientCount() != 1 {
		t.Errorf("expected 1 client after remove, got %d", room.ClientCount())
	}

	room.Remove(c2)
	if room.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", room.ClientCount())
	}
}

func TestRoom_Broadcast(t *testing.T) {
	room := NewRoom("test-room")

	c1 := &Client{peerID: "peer-1", connID: "conn-1", send: make(chan []byte, 10)}
	c2 := &Client{peerID: "peer-2", connID: "conn-2", send: make(chan []byte, 10)}
	c3 := &Client{peerID: "peer-3", connID: "conn-3", send: make(chan []byte, 10)}

	room.Add(c1)
	room.Add(c2)
	room.Add(c3)

	// Broadcast from c1 — should reach c2 and c3 but not c1
	room.Broadcast("conn-1", []byte("hello"))

	// Check c2 received
	select {
	case msg := <-c2.send:
		if string(msg) != "hello" {
			t.Errorf("c2 got %q, want %q", msg, "hello")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("c2 did not receive message")
	}

	// Check c3 received
	select {
	case msg := <-c3.send:
		if string(msg) != "hello" {
			t.Errorf("c3 got %q, want %q", msg, "hello")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("c3 did not receive message")
	}

	// Verify c1 did NOT receive (sender excluded)
	select {
	case <-c1.send:
		t.Error("sender c1 should not receive own broadcast")
	case <-time.After(50 * time.Millisecond):
		// OK — no message for sender
	}
}

func TestRoom_SamePeerID_DifferentConnID(t *testing.T) {
	room := NewRoom("test-room")

	c1 := &Client{peerID: "same-peer", connID: "conn-1", send: make(chan []byte, 10)}
	c2 := &Client{peerID: "same-peer", connID: "conn-2", send: make(chan []byte, 10)}

	room.Add(c1)
	room.Add(c2)

	if room.ClientCount() != 2 {
		t.Errorf("expected 2 clients with same peerID but different connID, got %d", room.ClientCount())
	}

	room.Broadcast("conn-1", []byte("hello"))

	select {
	case msg := <-c2.send:
		if string(msg) != "hello" {
			t.Errorf("c2 got %q, want %q", msg, "hello")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("c2 should receive message from c1 (different connID)")
	}

	select {
	case <-c1.send:
		t.Error("sender c1 should not receive own broadcast")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRoom_LastActivity(t *testing.T) {
	room := NewRoom("test-room")

	before := room.LastActivity()
	time.Sleep(10 * time.Millisecond)

	c := &Client{peerID: "peer-1", connID: "conn-1", send: make(chan []byte, 10)}
	room.Add(c)

	after := room.LastActivity()
	if !after.After(before) {
		t.Error("LastActivity should be updated after Add")
	}
}
