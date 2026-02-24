# From Scratch Setup (New Droplet or New Cloud)

This runbook rebuilds the MVP from zero on a fresh Linux VM.

## 1) Host Requirements
- Ubuntu 24.04 x86_64 (or equivalent modern Linux)
- KVM available: `/dev/kvm` exists and `kvm-ok` passes
- Root access
- Public internet egress

Provider notes:
- DigitalOcean: verify KVM availability immediately after boot.
- Other providers: use an instance type with hardware virtualization enabled.

## 2) Install Dependencies

```bash
apt-get update
apt-get install -y \
  git curl jq lvm2 cpu-checker bridge-utils iptables iproute2 qemu-utils unzip \
  debootstrap golang-go make
```

## 3) Clone Repository

```bash
git clone git@github.com:SteveLiang/agent_sandbox_app.git
cd agent_sandbox_app
```

## 4) Validate KVM + Bootstrap Host

```bash
bash scripts/bootstrap_host.sh
bash scripts/validate_kvm.sh
```

## 5) Create Thin-Pool Storage (Loopback MVP)

```bash
truncate -s 80G /var/lib/microvm/thinpool.img
LOOP_DEV=$(losetup --show -f /var/lib/microvm/thinpool.img)
pvcreate "$LOOP_DEV"
vgcreate microvm-vg "$LOOP_DEV"
lvcreate --type thin-pool -L 70G -n sandbox-thinpool microvm-vg
lvs -a -o lv_name,vg_name,lv_attr,lv_active,lv_path
```

## 6) Install Firecracker

```bash
ARCH=$(uname -m)
FC_VERSION=v1.8.0
curl -L -o /tmp/firecracker.tgz \
  "https://github.com/firecracker-microvm/firecracker/releases/download/${FC_VERSION}/firecracker-${FC_VERSION}-${ARCH}.tgz"
tar -xzf /tmp/firecracker.tgz -C /tmp
install -m 755 /tmp/release-${FC_VERSION}-${ARCH}/firecracker-${FC_VERSION}-${ARCH} /usr/local/bin/firecracker
firecracker --version
```

## 7) Build Base Image LV (`img-base`)

```bash
lvcreate -V 8G -T microvm-vg/sandbox-thinpool -n img-base
mkfs.ext4 /dev/microvm-vg/img-base
mkdir -p /mnt/img-base
mount /dev/microvm-vg/img-base /mnt/img-base
debootstrap --arch=amd64 --components=main,universe noble /mnt/img-base http://archive.ubuntu.com/ubuntu/
```

Prepare chroot and install tools:

```bash
mount -t proc proc /mnt/img-base/proc
mount --rbind /sys /mnt/img-base/sys
mount --rbind /dev /mnt/img-base/dev

chroot /mnt/img-base bash -lc '
set -euo pipefail
apt-get update
apt-get install -y curl git zsh python3 python3-pip nodejs npm openssh-server ca-certificates
npm install -g @openai/codex
codex --version
'
```

Install SSH authorized key into base image:

```bash
# use whichever pubkey exists on host
PUBKEY_PATH=/root/.ssh/id_ed25519.pub
ROOTFS_MOUNT=/mnt/img-base PUBKEY_PATH=$PUBKEY_PATH bash scripts/prepare_guest_ssh.sh
```

Add static network setup service in guest (from kernel `ip=` args):

```bash
cat > /mnt/img-base/usr/local/sbin/microvm-net-setup.sh <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
mask_to_prefix(){ IFS=. read -r a b c d <<< "$1"; n=0; for o in $a $b $c $d; do case "$o" in 255) n=$((n+8));;254) n=$((n+7));;252) n=$((n+6));;248) n=$((n+5));;240) n=$((n+4));;224) n=$((n+3));;192) n=$((n+2));;128) n=$((n+1));;0);;*) exit 1;; esac; done; echo "$n"; }
iparg=""; for tok in $(cat /proc/cmdline); do case "$tok" in ip=*) iparg="${tok#ip=}";; esac; done
[ -n "$iparg" ] || exit 0
IFS=':' read -r client _ gw mask _ dev _ <<< "$iparg"
[ -n "${dev:-}" ] || dev=eth0
prefix=$(mask_to_prefix "$mask")
ip link set "$dev" up
ip addr flush dev "$dev" || true
ip addr add "${client}/${prefix}" dev "$dev"
ip route replace default via "$gw" dev "$dev"
SCRIPT
chmod +x /mnt/img-base/usr/local/sbin/microvm-net-setup.sh

cat > /mnt/img-base/etc/systemd/system/microvm-net-setup.service <<'UNIT'
[Unit]
Description=Configure static network from kernel ip= cmdline
After=local-fs.target
Before=network.target ssh.service

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/microvm-net-setup.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
UNIT
mkdir -p /mnt/img-base/etc/systemd/system/multi-user.target.wants
ln -sf /etc/systemd/system/microvm-net-setup.service /mnt/img-base/etc/systemd/system/multi-user.target.wants/microvm-net-setup.service
```

Unmount cleanly:

```bash
umount -R /mnt/img-base/dev || true
umount -R /mnt/img-base/sys || true
umount /mnt/img-base/proc || true
umount /mnt/img-base
```

## 8) Build Agent + Install Persistent Services

```bash
go build -o bin/runtime-agent ./cmd/runtime-agent
bash scripts/install_storage_service.sh
bash scripts/install_networking_service.sh
bash scripts/install_runtime_agent_service.sh
```

Verify:

```bash
systemctl status microvm-storage.service --no-pager -l
systemctl status microvm-networking.service --no-pager -l
systemctl status runtime-agent.service --no-pager -l
curl -sS http://127.0.0.1:8081/healthz | jq
```

## 9) Run End-to-End Verification

```bash
bash scripts/e2e_verify.sh
```

Expected: final line `E2E PASS`.

## 10) Cost Control
- Create a provider snapshot/image after passing E2E.
- Power off and destroy runtime VM when idle.
- Restore from snapshot next day and rerun `bash scripts/e2e_verify.sh`.
