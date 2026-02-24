# microvm-sandbox-mvp

Minimal MVP for agent sandboxes with near-instant snapshot and restore semantics.

This project targets:
- Runtime host: Linux (Ubuntu 24.04+) with KVM
- Developer machine: macOS (control plane development)
- Sandboxes: microVM-based, with both Codex and Claude tooling available in each environment

## MVP Goals
- Create sandbox from a base image
- Snapshot sandbox filesystem quickly
- Restore snapshot into a new sandbox quickly
- Keep API and workflow simple and reproducible

## Repository Layout
- `docs/ARCHITECTURE.md`: system design and MVP boundaries
- `docs/WORKFLOW.md`: step-by-step build and validation workflow
- `scripts/validate_kvm.sh`: host capability checks
- `scripts/bootstrap_host.sh`: runtime host dependency bootstrap

## Quick Start
1. Read [docs/WORKFLOW.md](/Users/dreaminvm/agent_sandbox_app/docs/WORKFLOW.md)
2. On the droplet, run:
   - `bash scripts/bootstrap_host.sh`
   - `bash scripts/validate_kvm.sh`
3. Follow workflow phases to build base image, snapshot path, and control plane API.
