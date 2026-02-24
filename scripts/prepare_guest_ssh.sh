#!/usr/bin/env bash
set -euo pipefail

ROOTFS_MOUNT="${ROOTFS_MOUNT:-/mnt/img-base}"
PUBKEY_PATH="${PUBKEY_PATH:-$HOME/.ssh/id_rsa.pub}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root."
  exit 1
fi

if [[ ! -d "${ROOTFS_MOUNT}" ]]; then
  echo "Missing mount path: ${ROOTFS_MOUNT}"
  exit 1
fi
if [[ ! -f "${PUBKEY_PATH}" ]]; then
  echo "Missing public key: ${PUBKEY_PATH}"
  exit 1
fi

echo "Preparing SSH inside guest rootfs at ${ROOTFS_MOUNT}"

mountpoint -q "${ROOTFS_MOUNT}/proc" || mount -t proc proc "${ROOTFS_MOUNT}/proc"
mountpoint -q "${ROOTFS_MOUNT}/sys" || mount --rbind /sys "${ROOTFS_MOUNT}/sys"
mountpoint -q "${ROOTFS_MOUNT}/dev" || mount --rbind /dev "${ROOTFS_MOUNT}/dev"

chroot "${ROOTFS_MOUNT}" bash -lc '
set -euo pipefail
apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y openssh-server
mkdir -p /root/.ssh
chmod 700 /root/.ssh
'

install -m 600 "${PUBKEY_PATH}" "${ROOTFS_MOUNT}/root/.ssh/authorized_keys"

cat > "${ROOTFS_MOUNT}/etc/ssh/sshd_config.d/90-microvm.conf" <<'SSHD'
PermitRootLogin prohibit-password
PasswordAuthentication no
PubkeyAuthentication yes
SSHD

echo "SSH guest prep complete."
echo "Remember to unmount:"
echo "  umount -R ${ROOTFS_MOUNT}/dev || true"
echo "  umount -R ${ROOTFS_MOUNT}/sys || true"
echo "  umount ${ROOTFS_MOUNT}/proc || true"
echo "  umount ${ROOTFS_MOUNT}"
