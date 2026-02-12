package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestValidateJWT_Valid(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	claims := &Claims{
		RoomID:    "test-room",
		PeerID:    "test-peer",
		Role:      "host",
		Name:      "Alice",
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}

	token := SignJWT(claims, priv)

	auth := NewAuth()
	got, err := auth.ValidateJWT(token, pub)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}

	if got.RoomID != claims.RoomID {
		t.Errorf("room_id = %q, want %q", got.RoomID, claims.RoomID)
	}
	if got.PeerID != claims.PeerID {
		t.Errorf("peer_id = %q, want %q", got.PeerID, claims.PeerID)
	}
	if got.Role != claims.Role {
		t.Errorf("role = %q, want %q", got.Role, claims.Role)
	}
}

func TestValidateJWT_Expired(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	claims := &Claims{
		RoomID:    "test-room",
		PeerID:    "test-peer",
		Role:      "host",
		Name:      "Alice",
		CreatedAt: time.Now().Add(-48 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(-24 * time.Hour).Unix(), // Expired
	}

	token := SignJWT(claims, priv)

	auth := NewAuth()
	_, err := auth.ValidateJWT(token, pub)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateJWT_WrongKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)

	claims := &Claims{
		RoomID:    "test-room",
		PeerID:    "test-peer",
		Role:      "guest",
		Name:      "Bob",
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}

	token := SignJWT(claims, priv)

	auth := NewAuth()
	_, err := auth.ValidateJWT(token, otherPub)
	if err == nil {
		t.Fatal("expected error for wrong key")
	}
}

func TestValidateJWT_InvalidRole(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	claims := &Claims{
		RoomID:    "test-room",
		PeerID:    "test-peer",
		Role:      "admin", // Invalid role
		Name:      "Eve",
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}

	token := SignJWT(claims, priv)

	auth := NewAuth()
	_, err := auth.ValidateJWT(token, pub)
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestValidateJWT_MalformedToken(t *testing.T) {
	auth := NewAuth()

	_, err := auth.ValidateJWT("not.a.valid.token", make([]byte, 32))
	if err == nil {
		t.Fatal("expected error for malformed token")
	}

	_, err = auth.ValidateJWT("", make([]byte, 32))
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestValidateJWT_HostSignsForGuest(t *testing.T) {
	// Host generates keypair and signs a JWT for a guest
	hostPub, hostPriv, _ := ed25519.GenerateKey(rand.Reader)

	guestClaims := &Claims{
		RoomID:    "shared-room",
		PeerID:    "guest-1",
		Role:      "guest",
		Name:      "Bob",
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}

	// Host signs the guest JWT
	guestToken := SignJWT(guestClaims, hostPriv)

	// Relay verifies against host's public key
	auth := NewAuth()
	got, err := auth.ValidateJWT(guestToken, hostPub)
	if err != nil {
		t.Fatalf("relay should accept host-signed guest JWT: %v", err)
	}

	if got.Role != "guest" {
		t.Errorf("role = %q, want guest", got.Role)
	}
	if got.PeerID != "guest-1" {
		t.Errorf("peer_id = %q, want guest-1", got.PeerID)
	}
}
