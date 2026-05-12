#!/usr/bin/env bash
# Build wabot binaries. Optionally install wa/inbox-echo/wabot to PREFIX/bin
# and/or install systemd units (requires INSTALL_DIR, WABOT_USER).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

usage() {
	cat <<EOF
Usage: $0 [options]

  Builds: ./wabot ./wa ./inbox-echo in the repository root.

Options:
  --prefix DIR     sudo install binaries to DIR/bin (default: skip)
  --install-systemd
                   write systemd units from deploy/systemd/*.example
                   (needs INSTALL_DIR and WABOT_USER; uses sudo)
  -h, --help       show this help

Environment (for --install-systemd):
  INSTALL_DIR   absolute path where wabot, wabot.env, store.db live (e.g. /opt/wabot)
  WABOT_USER    unix user to run the service (e.g. wabot)

Example:
  ./scripts/install.sh
  ./scripts/install.sh --prefix /usr/local
  INSTALL_DIR=/opt/wabot WABOT_USER=wabot ./scripts/install.sh --install-systemd
EOF
}

PREFIX=""
INSTALL_SYSTEMD=false
while [[ $# -gt 0 ]]; do
	case "$1" in
	--prefix)
		PREFIX="${2:?}"
		shift 2
		;;
	--install-systemd)
		INSTALL_SYSTEMD=true
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "unknown option: $1" >&2
		usage
		exit 1
		;;
	esac
done

echo "==> Building (CGO required for wabot SQLite)..."
export CGO_ENABLED=1
go build -trimpath -ldflags="-s -w" -o "$ROOT/wabot" ./cmd/wabot
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$ROOT/wa" ./cmd/wa
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$ROOT/inbox-echo" ./cmd/inbox-echo
echo "    $ROOT/wabot"
echo "    $ROOT/wa"
echo "    $ROOT/inbox-echo"

if [[ -n "${PREFIX}" ]]; then
	echo "==> Installing binaries to ${PREFIX}/bin ..."
	sudo install -m 755 "$ROOT/wabot" "${PREFIX}/bin/wabot"
	sudo install -m 755 "$ROOT/wa" "${PREFIX}/bin/wa"
	sudo install -m 755 "$ROOT/inbox-echo" "${PREFIX}/bin/inbox-echo"
fi

if $INSTALL_SYSTEMD; then
	: "${INSTALL_DIR:?Set INSTALL_DIR to an absolute path (e.g. /opt/wabot)}"
	: "${WABOT_USER:?Set WABOT_USER (e.g. wabot)}"
	if [[ "${INSTALL_DIR}" != /* ]]; then
		echo "INSTALL_DIR must be an absolute path" >&2
		exit 1
	fi
	echo "==> Installing systemd units..."
	sed -e "s|@@INSTALL_DIR@@|${INSTALL_DIR}|g" -e "s|@@WABOT_USER@@|${WABOT_USER}|g" \
		"$ROOT/deploy/systemd/wabot.service.example" | sudo tee /etc/systemd/system/wabot.service >/dev/null
	sed -e "s|@@INSTALL_DIR@@|${INSTALL_DIR}|g" -e "s|@@WABOT_USER@@|${WABOT_USER}|g" \
		"$ROOT/deploy/systemd/wabot-backup-store.service.example" | sudo tee /etc/systemd/system/wabot-backup-store.service >/dev/null
	sudo install -m 644 "$ROOT/deploy/systemd/wabot-backup-store.timer.example" /etc/systemd/system/wabot-backup-store.timer
	sudo systemctl daemon-reload
	echo "    Enabled units: wabot.service, wabot-backup-store.timer"
	echo "    Run: sudo systemctl enable --now wabot.service"
	echo "    Run: sudo systemctl enable --now wabot-backup-store.timer"
fi

echo "==> Done."
