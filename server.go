package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  65536,
	WriteBufferSize: 65536,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Server struct {
	cfg     *Config
	hub     *Hub
	srv     *http.Server
	auth    *Auth
	limiter *RateLimiter
}

func NewServer(cfg *Config, hub *Hub) *Server {
	s := &Server{
		cfg:     cfg,
		hub:     hub,
		auth:    NewAuth(),
		limiter: NewRateLimiter(cfg.RateLimitPerIP),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ws", s.handleWS)

	s.srv = &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

func (s *Server) ListenAndServe() error {
	if s.cfg.TLSCert != "" && s.cfg.TLSKey != "" {
		s.srv.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS13,
		}
		log.Printf("TLS enabled (cert=%s)", s.cfg.TLSCert)
		return s.srv.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
	}
	log.Println("TLS disabled (no cert/key configured)")
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)

	if !s.limiter.Allow(ip) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	roomID := r.URL.Query().Get("room")
	token := r.URL.Query().Get("token")
	pubkey := r.URL.Query().Get("pubkey")

	if roomID == "" || token == "" {
		http.Error(w, "missing room or token", http.StatusBadRequest)
		return
	}

	// Host provides pubkey to register; guests don't
	isHost := pubkey != ""

	var claims *Claims
	var err error

	if isHost {
		hostPubKey, decErr := base64.RawURLEncoding.DecodeString(pubkey)
		if decErr != nil || len(hostPubKey) != 32 {
			http.Error(w, "invalid pubkey", http.StatusBadRequest)
			return
		}
		claims, err = s.auth.ValidateJWT(token, hostPubKey)
		if err != nil {
			http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}
		if claims.RoomID != roomID {
			http.Error(w, "room mismatch", http.StatusForbidden)
			return
		}
		s.hub.RegisterHostKey(roomID, hostPubKey)
	} else {
		hostKey := s.hub.GetHostKey(roomID)
		if hostKey == nil {
			http.Error(w, "room not found", http.StatusNotFound)
			return
		}
		claims, err = s.auth.ValidateJWT(token, hostKey)
		if err != nil {
			http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}
		if claims.RoomID != roomID {
			http.Error(w, "room mismatch", http.StatusForbidden)
			return
		}
	}

	if isHost {
		if s.hub.RoomCount() >= s.cfg.MaxRooms {
			http.Error(w, "max rooms reached", http.StatusServiceUnavailable)
			return
		}
	} else {
		if count := s.hub.ClientCount(roomID); count >= s.cfg.MaxClientsPerRoom {
			http.Error(w, "room full", http.StatusServiceUnavailable)
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	// Set generous read limit. Messages within MaxMessageSize are forwarded normally.
	// gorilla/websocket closes connection on exceeding ReadLimit, so set it high (50MB)
	// to prevent accidental disconnects. Client-side safety net drops messages > 8MB.
	conn.SetReadLimit(50 * 1024 * 1024)

	client := NewClient(s.hub, conn, roomID, claims.PeerID, claims.Role, ip)
	s.hub.Register(client)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
