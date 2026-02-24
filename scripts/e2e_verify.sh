#!/usr/bin/env bash
set -euo pipefail

RUNTIME_URL="${RUNTIME_URL:-http://127.0.0.1:8081}"
SANDBOX_A="${SANDBOX_A:-e2ea}"
SNAPSHOT_ID="${SNAPSHOT_ID:-e2esnap}"
SANDBOX_B="${SANDBOX_B:-e2eb}"

json() {
  jq -r "$1"
}

wait_for_ssh() {
  local ip="$1"
  local tries="${2:-60}"
  local label="${3:-guest}"
  for i in $(seq 1 "$tries"); do
    if timeout 1 bash -c "cat < /dev/null > /dev/tcp/${ip}/22" 2>/dev/null; then
      echo "[info] SSH ready on ${label} (${ip}) after ${i}s"
      return 0
    fi
    sleep 1
  done
  echo "[error] SSH not ready on ${label} (${ip}) after ${tries}s"
  return 1
}

echo "[0/9] cleanup old resources"
curl -sS -X POST "${RUNTIME_URL}/v1/sandboxes/stop" -H 'content-type: application/json' -d "{\"sandbox_id\":\"${SANDBOX_A}\"}" >/dev/null || true
curl -sS -X POST "${RUNTIME_URL}/v1/sandboxes/stop" -H 'content-type: application/json' -d "{\"sandbox_id\":\"${SANDBOX_B}\"}" >/dev/null || true
curl -sS -X DELETE "${RUNTIME_URL}/v1/sandboxes?sandbox_id=${SANDBOX_A}" >/dev/null || true
curl -sS -X DELETE "${RUNTIME_URL}/v1/sandboxes?sandbox_id=${SANDBOX_B}" >/dev/null || true
curl -sS -X DELETE "${RUNTIME_URL}/v1/snapshots?snapshot_id=${SNAPSHOT_ID}" >/dev/null || true

echo "[1/9] create A"
curl -sS -X POST "${RUNTIME_URL}/v1/sandboxes" -H 'content-type: application/json' -d "{\"sandbox_id\":\"${SANDBOX_A}\",\"size_gb\":8}" | jq

echo "[2/9] start A"
curl -sS -X POST "${RUNTIME_URL}/v1/sandboxes/start" -H 'content-type: application/json' -d "{\"sandbox_id\":\"${SANDBOX_A}\",\"vcpu_count\":2,\"mem_mib\":1024}" | jq

A_IP=$(curl -sS "${RUNTIME_URL}/v1/sandboxes/status?sandbox_id=${SANDBOX_A}" | json '.result.network.guest_ip')
wait_for_ssh "${A_IP}" 120 "A"

echo "[3/9] write marker before in A (${A_IP})"
ssh -o StrictHostKeyChecking=no root@"${A_IP}" 'echo before > /root/marker.txt && cat /root/marker.txt'

echo "[4/9] snapshot A"
curl -sS -X POST "${RUNTIME_URL}/v1/snapshots" -H 'content-type: application/json' -d "{\"sandbox_id\":\"${SANDBOX_A}\",\"snapshot_id\":\"${SNAPSHOT_ID}\"}" | jq

echo "[5/9] mutate marker in A"
ssh -o StrictHostKeyChecking=no root@"${A_IP}" 'echo after > /root/marker.txt && cat /root/marker.txt'

echo "[6/9] restore snapshot to B"
curl -sS -X POST "${RUNTIME_URL}/v1/restores" -H 'content-type: application/json' -d "{\"snapshot_id\":\"${SNAPSHOT_ID}\",\"sandbox_id\":\"${SANDBOX_B}\"}" | jq

echo "[7/9] start B"
curl -sS -X POST "${RUNTIME_URL}/v1/sandboxes/start" -H 'content-type: application/json' -d "{\"sandbox_id\":\"${SANDBOX_B}\",\"vcpu_count\":2,\"mem_mib\":1024}" | jq

B_IP=$(curl -sS "${RUNTIME_URL}/v1/sandboxes/status?sandbox_id=${SANDBOX_B}" | json '.result.network.guest_ip')
wait_for_ssh "${B_IP}" 120 "B"

echo "[8/9] verify marker in B (${B_IP})"
VALUE=$(ssh -o StrictHostKeyChecking=no root@"${B_IP}" 'cat /root/marker.txt')
echo "marker=${VALUE}"
if [[ "${VALUE}" != "before" ]]; then
  echo "E2E FAILED: expected 'before', got '${VALUE}'"
  exit 1
fi

echo "[9/9] sanity codex in B"
ssh -o StrictHostKeyChecking=no root@"${B_IP}" 'codex --version || true'

echo "E2E PASS"
