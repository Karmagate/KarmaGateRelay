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

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |
| < Latest | No      |

We recommend always running the latest release.
