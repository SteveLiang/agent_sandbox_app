#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root."
  exit 1
fi

REPO_DIR="${REPO_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
SERVICE_PATH="/etc/systemd/system/microvm-networking.service"

cat >"${SERVICE_PATH}" <<UNIT
[Unit]
Description=Configure microVM bridge/tap networking
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=${REPO_DIR}/scripts/setup_networking.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now microvm-networking.service
systemctl status microvm-networking.service --no-pager -l || true

echo "Installed ${SERVICE_PATH}"
