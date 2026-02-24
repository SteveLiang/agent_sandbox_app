#!/usr/bin/env bash
set -euo pipefail

RUNTIME_URL="${RUNTIME_URL:-http://127.0.0.1:8081}"

sandbox_id="sbx$(date +%s)"
snapshot_id="snap$(date +%s)"
restore_id="rsbx$(date +%s)"

echo "[1/5] health check"
curl -sS "${RUNTIME_URL}/healthz" | jq

echo "[2/5] create sandbox volume ${sandbox_id}"
curl -sS -X POST "${RUNTIME_URL}/v1/sandboxes" \
  -H 'content-type: application/json' \
  -d "{\"sandbox_id\":\"${sandbox_id}\",\"size_gb\":5}" | jq

echo "[3/5] snapshot volume ${sandbox_id} -> ${snapshot_id}"
curl -sS -X POST "${RUNTIME_URL}/v1/snapshots" \
  -H 'content-type: application/json' \
  -d "{\"sandbox_id\":\"${sandbox_id}\",\"snapshot_id\":\"${snapshot_id}\"}" | jq

echo "[4/5] restore snapshot ${snapshot_id} -> ${restore_id}"
curl -sS -X POST "${RUNTIME_URL}/v1/restores" \
  -H 'content-type: application/json' \
  -d "{\"snapshot_id\":\"${snapshot_id}\",\"sandbox_id\":\"${restore_id}\"}" | jq

echo "[5/5] cleanup"
curl -sS -X DELETE "${RUNTIME_URL}/v1/sandboxes?sandbox_id=${sandbox_id}" | jq
curl -sS -X DELETE "${RUNTIME_URL}/v1/sandboxes?sandbox_id=${restore_id}" | jq
curl -sS -X DELETE "${RUNTIME_URL}/v1/snapshots?snapshot_id=${snapshot_id}" | jq

echo "smoketest complete"
