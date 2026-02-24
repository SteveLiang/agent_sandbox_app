# Architecture (MVP)

## Objective
Deliver a local-development-friendly control plane and a Linux-hosted microVM runtime that supports fast filesystem snapshot and restore.

## Scope In
- One default base image with both `codex` and `claude` CLI available
- MicroVM create/delete
- Filesystem snapshot create/list/delete
- Restore snapshot to a new microVM
- Minimal audit log events and TTL cleanup

## Scope Out (for now)
- Process memory checkpoint/restore (CRIU)
- Multi-region scheduling
- Advanced tenancy/billing

## High-Level Components
1. Control Plane API (local dev on macOS)
- Handles sandbox and snapshot requests
- Persists metadata (Postgres)
- Dispatches async runtime jobs

2. Runtime Host (DigitalOcean Ubuntu + KVM)
- Runs microVMs
- Owns sandbox block devices and snapshot backend
- Executes create/snapshot/restore jobs

3. Storage Backend
- Block-level CoW snapshots (prefer LVM thin for MVP)
- Snapshot clone path used for restore-to-new-sandbox

4. Base Image/Template
- Ubuntu rootfs
- Shared developer tools
- Both `codex` and `claude` preinstalled
- First-boot hook to inject per-sandbox secrets/config

## Data Model (minimal)
- `sandboxes(id, state, template_id, volume_ref, created_at, ttl_at)`
- `snapshots(id, sandbox_id, volume_ref, created_at, size_bytes)`
- `templates(id, name, version, image_ref, created_at)`
- `audit_events(id, sandbox_id, event_type, payload, created_at)`

## API (minimal)
- `POST /sandboxes`
- `DELETE /sandboxes/{id}`
- `POST /sandboxes/{id}/snapshots`
- `GET /sandboxes/{id}/snapshots`
- `POST /snapshots/{id}/restore`

## Performance Targets (MVP)
- Snapshot p95: < 500ms
- Restore p95: < 1s
- Create from base p95: < 2s (after base image is warmed)

## Security Baseline
- Per-microVM resource caps
- Isolated network namespace per sandbox
- No persistent secret material in base snapshots by default
- Audit events for create/snapshot/restore/delete
