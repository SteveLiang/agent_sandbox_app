#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root."
  exit 1
fi

BRIDGE_IF="${BRIDGE_IF:-fcbr0}"
BRIDGE_CIDR="${BRIDGE_CIDR:-172.26.0.1/24}"
TAP_IF="${TAP_IF:-fctap0}"
WAN_IF="${WAN_IF:-$(ip route get 1.1.1.1 | awk '{print $5; exit}')}"
WAN_IF="$(echo "${WAN_IF}" | xargs)"

if [[ -z "${WAN_IF}" ]]; then
  echo "Unable to determine WAN_IF. Set WAN_IF explicitly."
  exit 1
fi

echo "Using BRIDGE_IF=${BRIDGE_IF} BRIDGE_CIDR=${BRIDGE_CIDR} TAP_IF=${TAP_IF} WAN_IF=${WAN_IF}"

if ! ip link show "${BRIDGE_IF}" >/dev/null 2>&1; then
  ip link add name "${BRIDGE_IF}" type bridge
fi

ip addr replace "${BRIDGE_CIDR}" dev "${BRIDGE_IF}"
ip link set "${BRIDGE_IF}" up

if ! ip tuntap show | awk '{print $1}' | grep -q "^${TAP_IF}:$"; then
  ip tuntap add dev "${TAP_IF}" mode tap
fi

ip link set "${TAP_IF}" master "${BRIDGE_IF}"
ip link set "${TAP_IF}" up

sysctl -w net.ipv4.ip_forward=1 >/dev/null
sysctl -w net.ipv4.conf.all.route_localnet=1 >/dev/null
sysctl -w net.ipv4.conf.default.route_localnet=1 >/dev/null

iptables -t nat -C POSTROUTING -s "${BRIDGE_CIDR%/*}" -o "${WAN_IF}" -j MASQUERADE 2>/dev/null || \
  iptables -t nat -A POSTROUTING -s "${BRIDGE_CIDR%/*}" -o "${WAN_IF}" -j MASQUERADE

iptables -C FORWARD -i "${BRIDGE_IF}" -o "${WAN_IF}" -j ACCEPT 2>/dev/null || \
  iptables -A FORWARD -i "${BRIDGE_IF}" -o "${WAN_IF}" -j ACCEPT

iptables -C FORWARD -i "${WAN_IF}" -o "${BRIDGE_IF}" -m state --state RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || \
  iptables -A FORWARD -i "${WAN_IF}" -o "${BRIDGE_IF}" -m state --state RELATED,ESTABLISHED -j ACCEPT

echo "Networking configured."
echo "Bridge: ${BRIDGE_IF}"
echo "Tap: ${TAP_IF}"
ip -br addr show dev "${BRIDGE_IF}"
ip -br addr show dev "${TAP_IF}"
