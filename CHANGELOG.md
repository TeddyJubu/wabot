# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-05-12

### Added

- Open source release under the MIT License.
- `wabot` daemon: WhatsApp (whatsmeow) + localhost HTTP `/send`, `/send-image`, `/health`.
- `wa` CLI for calling the daemon.
- `inbox-echo` helper for debugging inbound webhooks.
- Optional inbound webhook (`WABOT_INBOUND_URL`), rate limiting, send JSON log.
- Auto-reconnect after `StreamReplaced`; exit on logout / ban / outdated client.
- `scripts/install.sh`, `scripts/backup-store.sh`, and systemd unit templates under `deploy/systemd/`.
- Dockerfile and `docker-compose.yml` for container installs.
- GitHub Actions CI (`go test`, `go vet`).
