// E2E test: connects two WebSocket clients (host + guest) through a live relay.
// Usage: go run ./cmd/e2etest -relay ws://136.244.107.226:8443/ws
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

var relayURL = flag.String("relay", "ws://localhost:8443/ws", "relay WebSocket URL")

func main() {
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	// Generate host Ed25519 keypair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal("keygen:", err)
	}
	pubKeyB64 := base64.RawURLEncoding.EncodeToString(pubKey)

	roomID := "e2e-test-room"
	hostPeerID := "host-001"
	guestPeerID := "guest-001"

	// Sign host JWT
	hostJWT := signJWT(&claims{
		RoomID:    roomID,
		PeerID:    hostPeerID,
		Role:      "host",
		Name:      "Host",
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	}, privKey)

	// Sign guest JWT (host signs for guest)
	guestJWT := signJWT(&claims{
		RoomID:    roomID,
		PeerID:    guestPeerID,
		Role:      "guest",
		Name:      "Guest",
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	}, privKey)

	// --- Connect host ---
	log.Println(">> Connecting host...")
	hostParams := url.Values{
		"room":   {roomID},
		"pubkey": {pubKeyB64},
		"token":  {hostJWT},
	}
	hostConn, err := dial(*relayURL, hostParams)
	if err != nil {
		log.Fatal("host connect:", err)
	}
	defer hostConn.Close()
	log.Println("   Host connected ✓")

	// --- Connect guest ---
	log.Println(">> Connecting guest...")
	guestParams := url.Values{
		"room":  {roomID},
		"token": {guestJWT},
	}
	guestConn, err := dial(*relayURL, guestParams)
	if err != nil {
		log.Fatal("guest connect:", err)
	}
	defer guestConn.Close()
	log.Println("   Guest connected ✓")

	// --- Test: host sends message, guest receives ---
	testMsg := []byte(`{"type":"chat","text":"hello from host"}`)
	log.Println(">> Host sending message...")
	if err := hostConn.WriteMessage(websocket.BinaryMessage, testMsg); err != nil {
		log.Fatal("host send:", err)
	}
	log.Println("   Sent ✓")

	log.Println(">> Guest waiting for message...")
	_ = guestConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := guestConn.ReadMessage()
	if err != nil {
		log.Fatal("guest read:", err)
	}
	log.Printf("   Guest received: %s ✓", string(msg))

	// --- Test: guest sends message, host receives ---
	testMsg2 := []byte(`{"type":"chat","text":"hello from guest"}`)
	log.Println(">> Guest sending message...")
	if err := guestConn.WriteMessage(websocket.BinaryMessage, testMsg2); err != nil {
		log.Fatal("guest send:", err)
	}
	log.Println("   Sent ✓")

	log.Println(">> Host waiting for message...")
	_ = hostConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg2, err := hostConn.ReadMessage()
	if err != nil {
		log.Fatal("host read:", err)
	}
	log.Printf("   Host received: %s ✓", string(msg2))

	// --- Done ---
	fmt.Println()
	log.Println("═══════════════════════════════")
	log.Println("  E2E TEST PASSED ✓")
	log.Println("═══════════════════════════════")
	os.Exit(0)
}

func dial(baseURL string, params url.Values) (*websocket.Conn, error) {
	u := baseURL + "?" + params.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	return conn, err
}

type claims struct {
	RoomID    string `json:"room_id"`
	PeerID    string `json:"peer_id"`
	Role      string `json:"role"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

var jwtHeaderB64 = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))

func signJWT(c *claims, privKey ed25519.PrivateKey) string {
	claimsJSON, _ := json.Marshal(c)
	payloadB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := jwtHeaderB64 + "." + payloadB64
	sig := ed25519.Sign(privKey, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}
