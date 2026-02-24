# Runtime Agent (Droplet)

A minimal HTTP runtime adapter that wires sandbox volume lifecycle commands for:
- create sandbox volume
- snapshot sandbox volume
- restore snapshot to new sandbox volume
- delete sandbox/snapshot volume

This is intentionally small and command-driven so we can validate end-to-end before adding full microVM boot orchestration.

## Build

```bash
go build -o bin/runtime-agent ./cmd/runtime-agent
```

## Run (on droplet as root)

```bash
THIN_POOL=microvm-vg/sandbox-thinpool RUNTIME_ADDR=:8081 ./bin/runtime-agent
```

## API
- `GET /healthz`
- `POST /v1/sandboxes` body: `{"sandbox_id":"...","size_gb":5}`
- `DELETE /v1/sandboxes?sandbox_id=...`
- `GET /v1/sandboxes/status?sandbox_id=...`
- `POST /v1/sandboxes/start` body: `{"sandbox_id":"...","kernel_image":"/var/lib/microvm/images/vmlinux.bin","vcpu_count":2,"mem_mib":1024}`
- `POST /v1/sandboxes/stop` body: `{"sandbox_id":"..."}`
- `POST /v1/snapshots` body: `{"sandbox_id":"...","snapshot_id":"..."}`
- `DELETE /v1/snapshots?snapshot_id=...`
- `POST /v1/restores` body: `{"snapshot_id":"...","sandbox_id":"..."}`

## Smoke test

```bash
RUNTIME_URL=http://127.0.0.1:8081 bash scripts/runtime_agent_smoketest.sh
```

## Networking (host prep)

```bash
bash scripts/setup_networking.sh
```

Options:
- `BRIDGE_IF` default: `fcbr0`
- `BRIDGE_CIDR` default: `172.26.0.1/24`
- `TAP_IF` default: `fctap0`
- `WAN_IF` auto-detected from default route

## Notes
- IDs are validated against `[a-zA-Z0-9][a-zA-Z0-9_-]{2,63}`.
- This currently manages LVM thin volumes/snapshots.
- VM start clears LVM `activationskip` and force-activates the LV before drive attach.
- Firecracker process state lives under `/var/lib/microvm/sandboxes/<sandbox_id>/`.
