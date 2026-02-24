#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root (or via sudo)."
  exit 1
fi

echo "Installing host dependencies..."
apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y \
  cpu-checker \
  jq \
  curl \
  git \
  lvm2 \
  bridge-utils \
  iptables \
  iproute2 \
  qemu-utils \
  unzip

echo "Creating runtime directories..."
mkdir -p /opt/microvm/bin
mkdir -p /var/lib/microvm/{images,sandboxes,snapshots}

echo "Enabling IP forwarding for sandbox networking..."
cat >/etc/sysctl.d/99-microvm.conf <<'SYSCTL'
net.ipv4.ip_forward=1
SYSCTL
sysctl --system >/dev/null

echo "Bootstrap complete."
echo "Next steps:"
echo "1) Run scripts/validate_kvm.sh"
echo "2) Install/configure firecracker runtime binary"
echo "3) Configure LVM thin pool for snapshot backend"
