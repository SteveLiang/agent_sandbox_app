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
- `POST /v1/sandboxes/start` body: `{"sandbox_id":"...","kernel_image":"/var/lib/microvm/images/vmlinux.bin","vcpu_count":2,"mem_mib":1024,"bridge_if":"fcbr0","net_cidr":"172.26.0.0/24","tap_prefix":"fctap","ssh_port_min":2200,"ssh_port_max":2999}`
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
- If `tap_dev` is omitted, runtime-agent allocates a per-sandbox TAP from `tap_prefix` and attaches it to `bridge_if`.
- If `guest_mac` is omitted, runtime-agent generates a deterministic local MAC from sandbox ID.
- If `net_cidr` is set, runtime-agent allocates and persists a unique guest IP in sandbox network metadata.
- Runtime-agent sets static guest networking via kernel `ip=` boot args using allocated guest IP/gateway.
- Runtime-agent allocates and persists per-sandbox `ssh_port`, then adds localhost DNAT rule to guest `:22`.

## In-Guest Validation (SSH)

1. Start sandbox and capture status:

```bash
curl -sS -X POST http://127.0.0.1:8081/v1/sandboxes/start \
  -H 'content-type: application/json' \
  -d '{"sandbox_id":"demo1","vcpu_count":2,"mem_mib":1024}' | jq

curl -sS "http://127.0.0.1:8081/v1/sandboxes/status?sandbox_id=demo1" | jq
```

2. SSH using reported `network.ssh_port`:

```bash
ssh -p <ssh_port> -o StrictHostKeyChecking=no root@127.0.0.1
```
