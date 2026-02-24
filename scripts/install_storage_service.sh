#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root."
  exit 1
fi

REPO_DIR="${REPO_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
SERVICE_PATH="/etc/systemd/system/microvm-storage.service"
THINPOOL_IMG="${THINPOOL_IMG:-/var/lib/microvm/thinpool.img}"
VG_NAME="${VG_NAME:-microvm-vg}"

cat >"${SERVICE_PATH}" <<UNIT
[Unit]
Description=Attach microVM thinpool loopback and activate LVM VG
After=local-fs.target
Wants=local-fs.target

[Service]
Type=oneshot
ExecStart=/bin/bash -lc 'test -f ${THINPOOL_IMG}'
ExecStart=/bin/bash -lc 'losetup -j ${THINPOOL_IMG} | grep -q . || losetup -f ${THINPOOL_IMG}'
ExecStart=/sbin/pvscan --cache
ExecStart=/sbin/vgchange -ay ${VG_NAME}
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now microvm-storage.service
systemctl status microvm-storage.service --no-pager -l || true

echo "Installed ${SERVICE_PATH}"
