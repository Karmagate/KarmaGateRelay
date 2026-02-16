package main

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr              string
	TLSCert           string
	TLSKey            string
	MaxRooms          int
	MaxClientsPerRoom int
	MaxMessageSize    int64
	RoomIdleTimeout   time.Duration
	RateLimitPerIP    float64
	MetricsAddr       string
}

func LoadConfig() *Config {
	return &Config{
		Addr:              envStr("RELAY_ADDR", ":8443"),
		TLSCert:           envStr("RELAY_TLS_CERT", ""),
		TLSKey:            envStr("RELAY_TLS_KEY", ""),
		MaxRooms:          envInt("RELAY_MAX_ROOMS", 1000),
		MaxClientsPerRoom: envInt("RELAY_MAX_CLIENTS_PER_ROOM", 20),
		MaxMessageSize:    int64(envInt("RELAY_MAX_MESSAGE_SIZE", 52428800)),
		RoomIdleTimeout:   time.Duration(envInt("RELAY_ROOM_IDLE_TIMEOUT", 3600)) * time.Second,
		RateLimitPerIP:    float64(envInt("RELAY_RATE_LIMIT_PER_IP", 100)),
		MetricsAddr:       envStr("RELAY_METRICS_ADDR", ""),
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
