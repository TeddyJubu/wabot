# Security

## Reporting

Please report security issues **privately** via [GitHub Security Advisories](https://github.com/TeddyJubu/wabot/security/advisories/new) for this repository.

Do **not** post tokens, `store.db`, or message contents in public issues.

## Deployment notes

- The HTTP API is intended to bind to **loopback only** (`127.0.0.1:7777`) on a VPS. Use `WABOT_HTTP_ADDR` only when you understand the exposure (e.g. Docker port publishing with a reverse proxy).
- Protect `WABOT_TOKEN` like a password; anyone who can reach the socket and guess the token can send messages.
- WhatsApp session data lives in `store.db`; restrict filesystem permissions and back it up (see `scripts/backup-store.sh`).
