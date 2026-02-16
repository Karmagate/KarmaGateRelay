package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Auth struct{}

func NewAuth() *Auth {
	return &Auth{}
}

// Claims represents JWT payload for Bind sessions.
type Claims struct {
	RoomID    string `json:"room_id"`
	PeerID    string `json:"peer_id"`
	Role      string `json:"role"` // "host" or "guest"
	Name      string `json:"name"`
	CreatedAt int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// jwtHeader is the fixed header for Ed25519-signed JWTs.
var jwtHeaderB64 = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))

// ValidateJWT verifies a JWT signed with Ed25519 against the provided public key.
func (a *Auth) ValidateJWT(tokenStr string, pubKey []byte) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed JWT")
	}

	// Verify header
	if parts[0] != jwtHeaderB64 {
		return nil, errors.New("unsupported JWT algorithm")
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding: %w", err)
	}

	if len(pubKey) != ed25519.PublicKeySize {
		return nil, errors.New("invalid public key size")
	}

	if !ed25519.Verify(ed25519.PublicKey(pubKey), []byte(signingInput), sig) {
		return nil, errors.New("invalid signature")
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid claims encoding: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("invalid claims JSON: %w", err)
	}

	// Check expiry
	if claims.ExpiresAt > 0 && time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("token expired")
	}

	// Validate required fields
	if claims.RoomID == "" {
		return nil, errors.New("missing room_id")
	}
	if claims.PeerID == "" {
		return nil, errors.New("missing peer_id")
	}
	if claims.Role != "host" && claims.Role != "guest" {
		return nil, errors.New("invalid role")
	}

	return &claims, nil
}

// SignJWT creates a JWT signed with Ed25519 (used by clients, not relay).
// Included here for testing convenience.
func SignJWT(claims *Claims, privateKey ed25519.PrivateKey) string {
	claimsJSON, _ := json.Marshal(claims)
	payloadB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := jwtHeaderB64 + "." + payloadB64
	sig := ed25519.Sign(privateKey, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}
