#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root."
  exit 1
fi

REPO_DIR="${REPO_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
BIN_PATH="${BIN_PATH:-${REPO_DIR}/bin/runtime-agent}"
SERVICE_PATH="/etc/systemd/system/runtime-agent.service"

if [[ ! -x "${BIN_PATH}" ]]; then
  echo "Missing runtime-agent binary at ${BIN_PATH}. Build first: go build -o bin/runtime-agent ./cmd/runtime-agent"
  exit 1
fi

cat >"${SERVICE_PATH}" <<UNIT
[Unit]
Description=MicroVM Runtime Agent
After=network-online.target microvm-networking.service microvm-storage.service
Wants=network-online.target microvm-networking.service microvm-storage.service

[Service]
Type=simple
WorkingDirectory=${REPO_DIR}
Environment=THIN_POOL=microvm-vg/sandbox-thinpool
Environment=MICROVM_ROOT=/var/lib/microvm
Environment=KERNEL_IMAGE=/var/lib/microvm/images/vmlinux.bin
Environment=BASE_IMAGE_LV=/dev/microvm-vg/img-base
Environment=DEFAULT_BRIDGE_IF=fcbr0
Environment=DEFAULT_NET_CIDR=172.26.0.0/24
Environment=DEFAULT_TAP_PREFIX=fctap
Environment=DEFAULT_SSH_PORT_MIN=2200
Environment=DEFAULT_SSH_PORT_MAX=2999
Environment=RUNTIME_ADDR=:8081
ExecStart=${BIN_PATH}
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now runtime-agent.service
systemctl status runtime-agent.service --no-pager -l || true

echo "Installed ${SERVICE_PATH}"
