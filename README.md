# wabot

[![CI](https://github.com/TeddyJubu/wabot/actions/workflows/ci.yml/badge.svg)](https://github.com/TeddyJubu/wabot/actions/workflows/ci.yml)

WhatsApp daemon + local CLI for automation.

`wabot` keeps a persistent WhatsApp session. `wa` is the local client your scripts/agents call.

---

## Quickstart (3 commands)

```bash
git clone https://github.com/TeddyJubu/wabot.git
cd wabot && ./scripts/install.sh --prefix /usr/local
wa setup
```

Then test:

```bash
wa health
wa send 8801712345678 "hello from wabot"
```

---

## Why this is simpler now

- `wa setup` does interactive bootstrap:
  - generates token
  - writes token file (`~/.config/wabot/token`)
  - syncs `wabot.env`
  - builds/installs binaries
  - installs/enables systemd units (optional)
  - runs `wa doctor`
- `wa doctor` checks and auto-fixes common local misconfig:
  - missing token file
  - missing/mismatched token in `wabot.env`
  - stopped `wabot.service` (tries to start)
  - daemon reachability / auth probe
- Token complexity is hidden for local CLI:
  - `wa` uses `WABOT_TOKEN` if set
  - otherwise auto-loads token from `~/.config/wabot/token`

---

## Main commands

```bash
wa setup
wa doctor
wa health
wa send <number> "<message>"
wa send-image <number> <path> [caption]
```

Number format: country code + digits, no `+` or spaces (or full JID like `...@g.us`).

---

## Advanced

### Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `WABOT_ENDPOINT` | `http://127.0.0.1:7777` | Override daemon URL for CLI |
| `WABOT_TOKEN` | _(unset)_ | Explicit token override |
| `WABOT_TOKEN_FILE` | `~/.config/wabot/token` | Token file override |
| `WABOT_HTTP_ADDR` | `127.0.0.1:7777` | Daemon bind address |
| `WABOT_SEND_LOG` | `./sends.log` | JSONL send log |
| `WABOT_RATE_PER_MIN` | `20` | Rate limit refill |
| `WABOT_RATE_BURST` | `5` | Rate limit burst |
| `WABOT_INBOUND_URL` | _(unset)_ | Optional inbound webhook URL |
| `WABOT_INBOUND_TOKEN` | _(unset)_ | Bearer token for inbound webhook |
| `WABOT_INBOUND_TIMEOUT_SEC` | `10` | Inbound webhook timeout |

`wabot.env` is generated/managed by setup/doctor.

Production webhook URLs (loopback only — pair with [wabot-agent](https://github.com/TeddyJubu/wabot-agent)):

| Variable | Example |
|---|---|
| `WABOT_INBOUND_URL` | `http://127.0.0.1:8787/whatsapp/inbound` |
| `WABOT_RECEIPT_URL` | `http://127.0.0.1:8787/whatsapp/receipt` |
| `WABOT_PRESENCE_URL` | `http://127.0.0.1:8787/whatsapp/presence` |
| `WABOT_HISTORY_SYNC_URL` | `http://127.0.0.1:8787/whatsapp/history-sync` |
| `WABOT_HISTORY_URL` | `http://127.0.0.1:8787/whatsapp/history` |

Use the same bearer token for agent webhooks (`WABOT_INBOUND_TOKEN` on both sides).

### HTTP API

- `GET /health`
- `GET /pairing/qr` for browser-based linked-device setup
- `POST /send` with JSON `{ "to": "...", "text": "..." }`
- `POST /send-image` with multipart `to`, `file`, optional `caption`

Routes except `/health` require header `X-Token`.

`/pairing/qr` returns the latest fresh WhatsApp Web pairing code as JSON so a local dashboard can render a scan-able QR without asking the user to watch terminal logs. It is only available while the daemon is waiting for a linked-device login; already-linked sessions return linked status instead.

### systemd templates

Template units live in `deploy/systemd/` and are installed by:

```bash
INSTALL_DIR=/path/to/wabot WABOT_USER=youruser ./scripts/install.sh --install-systemd
```

### Backups

`scripts/backup-store.sh` rotates `store.db` backups.  
Timer template: `deploy/systemd/wabot-backup-store.timer.example`.

### Inbound webhook debugging

```bash
inbox-echo
```

Then set:

```bash
WABOT_INBOUND_URL=http://127.0.0.1:9000/whatsapp/inbound
```

### Docker

```bash
printf 'WABOT_TOKEN=%s\n' "$(openssl rand -hex 32)" > .env
docker compose up --build -d
```

---

## Development

```bash
go test ./...
go vet ./...
```

---

## Legal / safety

- This project is MIT licensed: see `LICENSE`.
- Dependencies include MPL-2.0 code (`go.mau.fi/whatsmeow`): see `THIRD_PARTY.md`.
- You are responsible for compliant use with WhatsApp terms/policies.
