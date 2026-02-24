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
