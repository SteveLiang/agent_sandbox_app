# microvm-sandbox-mvp

Minimal MVP for agent sandboxes with near-instant snapshot and restore semantics.

This project targets:
- Runtime host: Linux (Ubuntu 24.04+) with KVM
- Developer machine: macOS (control plane development)
- Sandboxes: microVM-based, with Codex tooling available in each environment

## MVP Goals
- Create sandbox from a base image
- Snapshot sandbox filesystem quickly
- Restore snapshot into a new sandbox quickly
- Keep API and workflow simple and reproducible

## Repository Layout
- `docs/ARCHITECTURE.md`: system design and MVP boundaries
- `docs/WORKFLOW.md`: step-by-step build and validation workflow
- `docs/FROM_SCRATCH.md`: full rebuild guide for new VM/provider
- `docs/RUNTIME_AGENT.md`: runtime adapter API and usage
- `scripts/validate_kvm.sh`: host capability checks
- `scripts/bootstrap_host.sh`: runtime host dependency bootstrap
- `scripts/runtime_agent_smoketest.sh`: exercises create/snapshot/restore/delete
- `scripts/prepare_guest_ssh.sh`: installs/keys SSH access in mounted guest rootfs
- `scripts/install_storage_service.sh`: persists thinpool loop device + VG activation on boot
- `scripts/e2e_verify.sh`: full SSH-based snapshot/restore verification
- `cmd/runtime-agent`: minimal droplet-side runtime adapter

## Quick Start
1. Read [docs/WORKFLOW.md](/Users/dreaminvm/agent_sandbox_app/docs/WORKFLOW.md)
2. On the droplet, run:
   - `bash scripts/bootstrap_host.sh`
   - `bash scripts/validate_kvm.sh`
3. Build and run runtime agent:
   - `go build -o bin/runtime-agent ./cmd/runtime-agent`
   - `THIN_POOL=microvm-vg/sandbox-thinpool RUNTIME_ADDR=:8081 ./bin/runtime-agent`
4. In another shell, run:
   - `RUNTIME_URL=http://127.0.0.1:8081 bash scripts/runtime_agent_smoketest.sh`
5. Follow remaining workflow phases for base image + control plane API.

## Rebuild Tomorrow
If you destroy your droplet and recreate later, follow:
- [docs/FROM_SCRATCH.md](/Users/dreaminvm/agent_sandbox_app/docs/FROM_SCRATCH.md)

This includes:
- fresh host bootstrap
- storage/runtime setup
- base image rebuild
- persistent services
- full E2E verification

## Roadmap To Sprites-Like Experience
Current state is runtime-first (API + microVM backend). To reach a user-facing account-to-terminal experience:

1. Identity and account model
- User registration/login
- API keys and workspace-level access control
- Multi-tenant authorization on every sandbox/snapshot operation

2. Control plane service
- Persistent metadata DB for users, workspaces, sandboxes, snapshots
- Job queue and state machine (create/start/snapshot/restore/stop)
- Audit logs and quota enforcement

3. Workspace UX
- Web app with “Create sandbox” and “Open terminal” actions
- Realtime terminal (websocket) and file browser
- Snapshot timeline with one-click restore

4. Agent-ready defaults
- One-click templates with codex-ready environment
- Secret injection at runtime (no secrets baked into snapshots)
- Per-workspace startup scripts

5. Reliability and scale
- Host pool scheduler and health checks
- Image caching and warm capacity for faster startup
- Metrics/SLO dashboards (create/snapshot/restore latency)

6. Billing and lifecycle
- Usage metering (runtime minutes, storage, snapshot count)
- TTL/auto-shutdown policies
- Payment and plan limits per workspace
