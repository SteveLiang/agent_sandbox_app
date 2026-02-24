# Workflow

This runbook captures the end-to-end workflow for bringing up the MVP.

## Phase 1: Host Readiness (DigitalOcean)
1. SSH to host:
- `ssh root@146.190.51.22`

2. Bootstrap dependencies (inside repo checkout on host):
- `bash scripts/bootstrap_host.sh`

3. Validate virtualization support:
- `bash scripts/validate_kvm.sh`

Pass criteria:
- `/dev/kvm` exists
- KVM acceleration available
- CPU virt flags present

## Phase 2: Runtime and Storage Setup
1. Configure microVM runtime service process (Firecracker adapter).
2. Configure LVM thin pool for sandbox block volumes.
3. Add runtime commands for:
- create volume
- create snapshot
- clone snapshot
- delete volume/snapshot

## Phase 3: Base Image Creation
1. Build one Ubuntu-based rootfs image.
2. Install shared toolchain (`git`, `curl`, `zsh`, `python3`, `node`, etc.).
3. Install both CLI tools:
- `codex`
- `claude`
4. Add first-boot hook for per-sandbox config/env injection.

Validation:
- New microVM can run `codex --help` and `claude --help`.

## Phase 4: Control Plane (macOS)
1. Implement API endpoints:
- `POST /sandboxes`
- `DELETE /sandboxes/{id}`
- `POST /sandboxes/{id}/snapshots`
- `POST /snapshots/{id}/restore`
2. Persist metadata in Postgres.
3. Add async job worker for runtime operations.

## Phase 5: End-to-End Test
1. Create sandbox.
2. Write known file in sandbox.
3. Snapshot.
4. Modify file.
5. Restore snapshot to new sandbox.
6. Verify restored sandbox has pre-modification state.

## Phase 6: MVP Hardening
1. Add TTL reaper for stale sandboxes/snapshots.
2. Add resource enforcement and failure handling.
3. Track p95 latency metrics for snapshot/restore.

## Definition of Done
- End-to-end snapshot/restore flow works consistently.
- Both `codex` and `claude` are available in every sandbox.
- Workflow is reproducible using this repository docs/scripts.
