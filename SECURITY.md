# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in KarmaGate Relay, please report it responsibly.

**Email:** security@karmagate.com

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

We will acknowledge your report within 48 hours and provide an estimated timeline for a fix.

## Security Design

KarmaGate Relay is designed as a **zero-knowledge relay**:

- All message payloads are end-to-end encrypted (XChaCha20-Poly1305) between clients
- The relay never sees plaintext content
- JWT authentication uses Ed25519 signatures verified against host public keys
- Every message is signed by the sender (Ed25519)
- Ephemeral session keys provide forward secrecy
- TLS 1.3 minimum for transport security

### Synthetic `session:leave` on disconnect

When a WebSocket drops, the relay may broadcast a **plaintext skeleton** envelope:

```json
{"type":"session:leave","from":"<peer_id>", ...}
```

This is **routing metadata only** (peer id + timestamp). It is not an E2E-authenticated End Session command.

**Client contract (KarmaGate Bind):**

- Treat Creator disconnect / synthetic leave as **`host_away`** — guests keep working.
- Only an explicit E2E `session:end` (or equivalent) dissolves the session for everyone.
- Do not treat unsigned/plaintext leave as authorization to wipe local projects.

### Host key grace and reclaim

**Ephemeral ether / durable operation:** empty rooms and retained host public keys do **not** live forever on the relay. Project data and creator resume secrets live only on KarmaGate clients.

| State | Guest JWT join | Host (pubkey + self JWT) |
|-------|----------------|---------------------------|
| Room has clients | Yes (valid JWT vs host key) | Same pubkey reconnect OK; **different** pubkey → `403 host key conflict` |
| Empty, within `RELAY_HOST_KEY_GRACE` (default 15m) | Yes | Same pubkey reclaim OK; different → conflict |
| Empty, after grace | `404 room not found` | Reclaim with **original** creator keys re-registers room |
| After idle cleanup | Same as after grace | Same |

After the last client leaves, the host Ed25519 **public** key may be retained for `RELAY_HOST_KEY_GRACE`. No session ciphertext, chat, or private keys are stored.

`RELAY_ROOM_IDLE_TIMEOUT` is clamped to be ≥ host-key grace so idle cleanup cannot drop keys earlier than the guest grace window.

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |
| < Latest | No      |

We recommend always running the latest release.
