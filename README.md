# wabot

[![CI](https://github.com/TeddyJubu/wabot/actions/workflows/ci.yml/badge.svg)](https://github.com/TeddyJubu/wabot/actions/workflows/ci.yml)

**wabot** is a small [WhatsApp](https://www.whatsapp.com/) **multi-device** bot daemon built on **[whatsmeow](https://github.com/tulir/whatsmeow)** (Go). It exposes a **localhost-only HTTP API** so scripts and AI agents can send messages quickly, plus an optional **inbound webhook** for replies.

> **Disclaimer:** WhatsApp’s terms, rate limits, and anti-abuse systems apply. This software is for **personal and educational** use. You are responsible for compliant use. The authors are not affiliated with WhatsApp.

| Component | Role |
|-----------|------|
| `wabot` | Long-lived process: WhatsApp session + HTTP API |
| `wa` | CLI: `wa send …`, `wa send-image …`, `wa health` |
| `inbox-echo` | Tiny debug server that prints inbound webhook POST bodies |

**License:** [MIT](LICENSE) for this repository. **Dependencies** include MPL-2.0 code (whatsmeow); see [THIRD_PARTY.md](THIRD_PARTY.md).

---

## Requirements

- **Go 1.25+** ([downloads](https://go.dev/dl/))
- **GCC + libc** on Linux for CGO (`github.com/mattn/go-sqlite3`), e.g. Debian/Ubuntu:

  ```bash
  sudo apt-get update && sudo apt-get install -y build-essential
  ```

- A **phone** with WhatsApp to scan the QR code on first pairing.

---

## Quick install (from source)

```bash
git clone https://github.com/TeddyJubu/wabot.git
cd wabot
cp .env.example wabot.env
chmod 600 wabot.env
# Put a long random secret in wabot.env (see below)
./scripts/install.sh
```

Generate a token:

```bash
openssl rand -hex 32
# paste into wabot.env as WABOT_TOKEN=...
```

**First run (QR pairing)** — run in a terminal (not detached) so you can scan the QR:

```bash
set -a && source wabot.env && set +a
./wabot
```

After you see **Successfully authenticated**, stop with `Ctrl+C`, then install **systemd** (recommended):

```bash
./scripts/install.sh --prefix /usr/local
INSTALL_DIR="$(pwd)" WABOT_USER="$USER" ./scripts/install.sh --install-systemd
sudo systemctl enable --now wabot.service
sudo systemctl enable --now wabot-backup-store.timer
```

Use absolute `INSTALL_DIR` if your checkout is not the service home (templates live in [`deploy/systemd/`](deploy/systemd/)).

---

## Environment variables

| Variable | Required | Default | Meaning |
|----------|----------|---------|---------|
| `WABOT_TOKEN` | **yes** | — | Shared secret; HTTP clients must send header `X-Token: <value>`. |
| `WABOT_HTTP_ADDR` | no | `127.0.0.1:7777` | HTTP listen address. Use `0.0.0.0:7777` only in Docker or behind a trusted proxy. |
| `WABOT_SEND_LOG` | no | `./sends.log` | Append-only JSON log of API sends. |
| `WABOT_RATE_PER_MIN` | no | `20` | Token bucket refill rate (messages per minute). Set to `0` with burst `0` to disable. |
| `WABOT_RATE_BURST` | no | `5` | Burst size. |
| `WABOT_INBOUND_URL` | no | — | HTTPS/HTTP URL for inbound JSON POSTs (see below). |
| `WABOT_INBOUND_TOKEN` | no | — | If set, sends `Authorization: Bearer …` to the inbound URL. |
| `WABOT_INBOUND_TIMEOUT_SEC` | no | `10` | Per-request timeout for the inbound webhook. |

Backup script ([`scripts/backup-store.sh`](scripts/backup-store.sh)) also honors:

| Variable | Default | Meaning |
|----------|---------|---------|
| `WABOT_DIR` | parent of `scripts/` | Project root. |
| `WABOT_STORE_DB` | `$WABOT_DIR/store.db` | SQLite session file. |
| `WABOT_BACKUP_DIR` | `$WABOT_DIR/backups` | Backup directory. |
| `WABOT_BACKUP_RETAIN` | `14` | Keep this many newest `store-*.db` files. |

---

## HTTP API (localhost)

All mutating routes require header **`X-Token: $WABOT_TOKEN`**.

| Method | Path | Body | Response |
|--------|------|------|----------|
| `GET` | `/health` | — | `{"connected":bool,"logged_in":bool}` |
| `POST` | `/send` | `{"to":"…","text":"…"}` | `{"id","timestamp","to"}` |
| `POST` | `/send-image` | `multipart/form-data`: `to`, `file`, optional `caption` | `{"id","timestamp","to","mime","bytes"}` |

- **`to`:** E.164 digits only (e.g. `8801712345678`) or full JID (e.g. group `...@g.us`).
- **Bind:** default is loopback only so random internet clients cannot reach the API.

---

## `wa` CLI

Install with `./scripts/install.sh --prefix /usr/local` or copy the `wa` binary to your `PATH`.

```bash
export WABOT_TOKEN=…   # same value as in wabot.env
wa health
wa send 8801712345678 "Hello from CI"
wa send-image 8801712345678 ./screenshot.png "caption"
```

- **`WABOT_ENDPOINT`:** override daemon base URL (default `http://127.0.0.1:7777`).
- Multi-word messages **must** be quoted; extra argv is rejected with a clear error.

---

## Inbound webhook (optional)

When `WABOT_INBOUND_URL` is set, every **incoming** message (not from you) triggers a `POST` with JSON:

```json
{
  "id": "message-id",
  "timestamp": "2026-05-12T12:00:00.123456789Z",
  "from": "sender@s.whatsapp.net",
  "chat": "chat@s.whatsapp.net",
  "is_group": false,
  "push_name": "Name",
  "text": "hello"
}
```

Debug locally:

```bash
inbox-echo &
export INBOX_ECHO_TOKEN=dev
export WABOT_INBOUND_URL=http://127.0.0.1:9000/whatsapp/inbound
export WABOT_INBOUND_TOKEN=dev
set -a && source wabot.env && set +a
./wabot
```

---

## Docker

From the repo root:

```bash
printf 'WABOT_TOKEN=%s\n' "$(openssl rand -hex 32)" > .env
chmod 600 .env
docker compose up --build -d
```

- API: `http://127.0.0.1:7777` on the host (mapped to the container).
- Session and DB live in the **`wabot-data`** Docker volume under `/data` inside the container.

---

## Operations

- **Backups:** `scripts/backup-store.sh` copies `store.db` with rotation. Systemd timer template: [`deploy/systemd/wabot-backup-store.timer.example`](deploy/systemd/wabot-backup-store.timer.example).
- **Reconnect:** the daemon handles `StreamReplaced` and transient disconnects; **logout / ban / outdated client** exits non-zero so **systemd** can restart (you may need to re-pair after logout).
- **Logs:** `journalctl -u wabot -f` (systemd) or stdout (foreground).

---

## Upgrading

Older checkouts shipped systemd units under `deploy/` with hardcoded paths. Current templates live in [`deploy/systemd/`](deploy/systemd/) as `*.example` files. Re-install units with:

```bash
INSTALL_DIR=/path/to/wabot WABOT_USER=youruser ./scripts/install.sh --install-systemd
sudo systemctl daemon-reload
```

---

## Development

```bash
go test ./...
go vet ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Repository

- **Home:** [github.com/TeddyJubu/wabot](https://github.com/TeddyJubu/wabot)
- **Security:** [SECURITY.md](SECURITY.md)
- **Changelog:** [CHANGELOG.md](CHANGELOG.md)
