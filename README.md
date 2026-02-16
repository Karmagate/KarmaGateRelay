<h1 align="center">
  <br>
  <a href="https://karmagate.com">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/Karmagate/App/main/logo-dark.svg">
      <source media="(prefers-color-scheme: light)" srcset="https://raw.githubusercontent.com/Karmagate/App/main/logo-light.svg">
      <img src="https://raw.githubusercontent.com/Karmagate/App/main/logo-dark.svg" width="200px" alt="KarmaGate">
    </picture>
  </a>
  <br>
  <br>
  KarmaGate Relay
  <br>
</h1>

<h4 align="center">Lightweight, stateless WebSocket relay server for <a href="https://karmagate.com">KarmaGate</a> Bind — real-time collaboration and voice chat for security teams.</h4>

<p align="center">
  <a href="https://github.com/Karmagate/KarmaGateRelay/releases/latest">
    <img src="https://img.shields.io/github/v/release/Karmagate/KarmaGateRelay?style=flat-square&color=00d4aa" alt="Latest Release"/>
  </a>
  <a href="https://github.com/Karmagate/KarmaGateRelay/releases">
    <img src="https://img.shields.io/github/downloads/Karmagate/KarmaGateRelay/total?style=flat-square&color=7c3aed" alt="Total Downloads"/>
  </a>
  <a href="https://karmagate.com">
    <img src="https://img.shields.io/badge/website-karmagate.com-blue?style=flat-square" alt="Website"/>
  </a>
</p>

<p align="center">
  <a href="#what-is-this">What is this</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#deployment">Deployment</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#license">License</a>
</p>

<br>

---

<br>

## What is this

KarmaGate Relay is the server component for **KarmaGate Bind** — real-time collaboration and encrypted voice chat between KarmaGate desktop clients.

The relay is a **dumb pipe**: it routes encrypted WebSocket messages and voice packets between peers. It never reads, stores, or decrypts session content. All data is end-to-end encrypted (XChaCha20-Poly1305) and voice is Opus-encoded with E2E encryption between clients.

| What relay sees | What relay does NOT see |
|----------------|----------------------|
| Room IDs, peer IDs | Request/response content |
| Message sizes, timing | Session secrets, encryption keys |
| IP addresses | Chat messages, findings |
| Voice packet sizes | Voice audio content (Opus E2E) |

**You don't need to self-host.** KarmaGate Pro includes access to `relay.karmagate.com`. Self-hosting is for organizations that require on-premise infrastructure.

<br>

---

<br>

## Quick Start

### Docker Compose (recommended)

```bash
git clone https://github.com/Karmagate/KarmaGateRelay.git
cd KarmaGateRelay
mkdir -p certs
# Add your TLS certs (see Deployment section below)
docker compose up -d --build
```

### Build from source

```bash
git clone https://github.com/Karmagate/KarmaGateRelay.git
cd KarmaGateRelay
go build -o relay .
./relay
```

### Connect KarmaGate

In KarmaGate desktop:

```
Settings → Bind → Relay Server
  [ ] Use default (relay.karmagate.com)
  [x] Custom: wss://your-relay.example.com:8443
```

<br>

---

<br>

## Deployment

### Requirements

- **OS**: Linux (Ubuntu 22+, Debian 12+, Alpine)
- **Docker** + Docker Compose (or Go 1.25+ to build from source)
- **TLS certificate** (Cloudflare Origin, Let's Encrypt, or custom CA)
- **Open port**: 8443 (or 443 behind reverse proxy)
- **Hardware**: 1 vCPU, 256 MB RAM, 1 GB disk (handles ~500 concurrent rooms)

### Step 1: Prepare the server

```bash
# Update system
apt update && apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Firewall
sudo ufw allow 22/tcp
sudo ufw allow 8443/tcp
sudo ufw enable
```

### Step 2: Clone and configure

```bash
git clone https://github.com/Karmagate/KarmaGateRelay.git
cd KarmaGateRelay
mkdir -p certs
```

### Step 3: TLS certificates

Choose one of the options below:

<details>
<summary><b>Option A: Cloudflare Origin Certificate (recommended with Cloudflare)</b></summary>

<br>

Best if your domain is already on Cloudflare. The Origin Certificate secures the connection between Cloudflare and your server, while Cloudflare handles the public-facing TLS.

**In Cloudflare Dashboard:**

1. Go to **SSL/TLS → Overview** → set mode to **Full (strict)**
2. Go to **SSL/TLS → Origin Server** → **Create Certificate**
   - Key type: **RSA (2048)** or **ECC**
   - Hostnames: `relay.yourdomain.com`
   - Validity: **15 years**
3. Copy the **Origin Certificate** and **Private Key**
4. Go to **DNS** → Add record:
   - Type: **A**
   - Name: `relay`
   - Content: your server IP
   - Proxy: **ON** (orange cloud)

> Cloudflare supports WebSocket on port 8443 with proxy enabled.

**On the server:**

```bash
nano certs/cert.pem   # paste Origin Certificate
nano certs/key.pem    # paste Private Key
chmod 600 certs/*.pem
```

</details>

<details>
<summary><b>Option B: Let's Encrypt (free, auto-renewing)</b></summary>

<br>

Best if you're not using Cloudflare proxy or need a universally trusted certificate.

> DNS A record must point directly to your server (no Cloudflare proxy).

```bash
# Install certbot
apt install -y certbot

# Get certificate (port 80 must be open temporarily)
sudo ufw allow 80/tcp
sudo certbot certonly --standalone -d relay.yourdomain.com
sudo ufw delete allow 80/tcp

# Copy certs
cp /etc/letsencrypt/live/relay.yourdomain.com/fullchain.pem certs/cert.pem
cp /etc/letsencrypt/live/relay.yourdomain.com/privkey.pem certs/key.pem
chmod 600 certs/*.pem
```

**Auto-renewal** — add a cron job:

```bash
crontab -e
```

```
0 3 * * * certbot renew --quiet && cp /etc/letsencrypt/live/relay.yourdomain.com/fullchain.pem /home/$USER/KarmaGateRelay/certs/cert.pem && cp /etc/letsencrypt/live/relay.yourdomain.com/privkey.pem /home/$USER/KarmaGateRelay/certs/key.pem && cd /home/$USER/KarmaGateRelay && docker compose restart
```

</details>

<details>
<summary><b>Option C: No TLS (behind reverse proxy)</b></summary>

<br>

If your relay sits behind nginx or Caddy that handles TLS termination:

```bash
# Remove TLS env vars from docker-compose.yml, then:
docker compose up -d --build
```

Example **nginx** config:

```nginx
server {
    listen 443 ssl;
    server_name relay.yourdomain.com;

    ssl_certificate     /etc/letsencrypt/live/relay.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/relay.yourdomain.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8443;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 86400s;
    }
}
```

</details>

### Step 4: Launch

```bash
docker compose up -d --build
```

### Step 5: Verify

```bash
# Check container is running
docker compose ps

# Check logs
docker compose logs

# Health check (from server)
curl -k https://localhost:8443/health
# → {"status":"ok"}

# Health check (from outside)
curl https://relay.yourdomain.com:8443/health
# → {"status":"ok"}
```

### Updating

```bash
cd ~/KarmaGateRelay
git pull
docker compose down
docker compose up -d --build
```

<br>

---

<br>

## Configuration

All configuration via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `RELAY_ADDR` | `:8443` | Listen address |
| `RELAY_TLS_CERT` | — | Path to TLS certificate |
| `RELAY_TLS_KEY` | — | Path to TLS private key |
| `RELAY_MAX_ROOMS` | `1000` | Maximum concurrent rooms |
| `RELAY_MAX_CLIENTS_PER_ROOM` | `20` | Maximum clients per room |
| `RELAY_MAX_MESSAGE_SIZE` | `1048576` | Maximum WebSocket message size (bytes) |
| `RELAY_ROOM_IDLE_TIMEOUT` | `3600` | Room idle timeout (seconds) |
| `RELAY_RATE_LIMIT_PER_IP` | `100` | WebSocket connections per second per IP |
| `RELAY_METRICS_ADDR` | — | Prometheus metrics address (e.g. `:9090`) |

### Docker Compose

```yaml
services:
  relay:
    build: .
    ports:
      - "8443:8443"
    volumes:
      - ./certs:/certs:ro
    environment:
      RELAY_ADDR: ":8443"
      RELAY_TLS_CERT: /certs/cert.pem
      RELAY_TLS_KEY: /certs/key.pem
    restart: unless-stopped
```

<br>

---

<br>

## Architecture

```
~1000 lines of Go. One binary. Zero external state.

┌──────────────────────────────────────┐
│           KarmaGate Relay            │
│                                      │
│  /health  → HTTP health check        │
│  /ws      → WebSocket upgrade        │
│                                      │
│  Hub ─── Room ─── Client             │
│   │       │        ├─ ReadPump       │
│   │       │        └─ WritePump      │
│   │       │             ├─ Voice (individual frames) │
│   │       │             └─ Data (batched)            │
│   │       └─ Broadcast (fan-out)     │
│   └─ Room lifecycle + cleanup        │
│                                      │
│  Auth: Ed25519 JWT verification      │
│  Rate Limiter: per-IP token bucket   │
└──────────────────────────────────────┘
```

### Security

- **E2E Encryption**: Relay never sees plaintext. All data payloads are XChaCha20-Poly1305 encrypted between clients.
- **Voice E2E**: Opus-encoded voice packets are encrypted with XChaCha20-Poly1305. Relay routes opaque binary frames — it cannot hear or decode audio.
- **Host-signed JWT**: Only the session host can issue access tokens. Relay verifies JWT signatures against the host's Ed25519 public key. No shared secrets.
- **Message signing**: Every message is Ed25519-signed by the sender. Relay cannot forge or tamper with messages.
- **Forward secrecy**: All session keys are ephemeral, stored in RAM only, and zeroed on session end.
- **TLS 1.3**: All connections use TLS 1.3 minimum.
- **Rate limiting**: Per-IP token bucket prevents abuse.

### Voice

Voice packets are identified by a 2-byte magic header (`0x4B56`) and are always sent as **individual WebSocket binary frames** — never batched with data messages. This ensures low-latency delivery and prevents corruption of encrypted binary payloads that may contain newline bytes.

### Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Returns `{"status":"ok"}` |
| `/ws` | GET (Upgrade) | WebSocket connection (data + voice). Query params: `room`, `token`, `pubkey` (host only) |

<br>

---

<br>

## Support

- **Email**: support@karmagate.com
- **Website**: [karmagate.com](https://karmagate.com)
- **Documentation**: [docs.karmagate.com](https://docs.karmagate.com)
- **Bug Reports**: [GitHub Issues](https://github.com/Karmagate/KarmaGateRelay/issues)

<br>

---

<br>

## License

Business Source License 1.1 (BSL 1.1)

**You may freely use this software** for self-hosting, internal security testing, personal use, education, and research. You may modify the source code for your own internal use.

**You may not** offer this as a commercial relay service or incorporate it into a competing security testing product.

After 4 years from each release, the code converts to Apache License 2.0.

See [LICENSE](LICENSE) for full details.

<br>

---

<br>

<div align="center">
  <sub>Built with love by the <a href="https://karmagate.com">KarmaGate</a> Team</sub>
  <br>
  <br>
  <a href="https://karmagate.com">Website</a> •
  <a href="https://t.me/karmagate">Telegram</a> •
  <a href="https://discord.gg/karmagate">Discord</a>
</div>
