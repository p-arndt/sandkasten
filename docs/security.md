# Security Guide

This document explains how to run Sandkasten with a hardened baseline and how to verify the most important controls.

## Hardened `sandkasten.yaml`

Use this as a production-oriented starting point and adapt for your environment:

```yaml
listen: "127.0.0.1:8080"
api_key: "sk-replace-with-long-random-secret"

data_dir: "/var/lib/sandkasten"
db_path: "/var/lib/sandkasten/sandkasten.db"

default_image: "python"
allowed_images:
  - "python"

session_ttl_seconds: 1800

defaults:
  cpu_limit: 1.0
  mem_limit_mb: 512
  pids_limit: 256
  max_exec_timeout_ms: 120000
  network_mode: "none"
  readonly_rootfs: true

workspace:
  enabled: true
  persist_by_default: false

security:
  seccomp: "strict"
```

## Why These Values

- `listen: 127.0.0.1:8080` keeps the daemon local; expose it via a hardened reverse proxy if needed.
- `api_key` is mandatory for any non-local setup. Use a long random value and rotate regularly.
- `allowed_images` limits execution to trusted images only.
- `network_mode: none` blocks sandbox egress and significantly reduces exfiltration risk.
- `readonly_rootfs: true` prevents writes outside writable mounts (`/workspace`, `/tmp`, runtime dirs).
- `security.seccomp: strict` blocks dangerous syscall classes beyond `mvp`.
- Resource limits (`cpu`, `memory`, `pids`, `max_exec_timeout_ms`) reduce DoS blast radius.

## Seccomp Profiles

`security.seccomp` supports three values:

- `off`: no seccomp syscall filter is applied. Useful only for debugging and compatibility checks.
- `mvp`: applies a deny-list filter for high-risk syscalls often used in kernel attack chains.
- `strict`: includes everything in `mvp` and additionally blocks namespace-changing syscalls for stronger isolation.

Current behavior in Sandkasten:

- `mvp` blocks syscall classes such as BPF, `userfaultfd`, `perf_event_open`, `ptrace`, module loading, keyring APIs, and mount/pivot operations.
- `strict` adds extra restrictions like `setns` and `unshare`.
- Blocked syscalls return `EPERM` inside the sandbox.

Recommended setting:

- Production: `strict`
- Staging/testing: `mvp` (or `strict` if your workloads are compatible)
- Local debugging only: `off`

## Security Validation Command

> [!IMPORTANT]
> Run the built-in baseline checker after any config change:

```bash
./bin/sandkasten security --config sandkasten.yaml
```

It verifies high-impact controls such as:

- API key posture vs listen address
- seccomp profile enabled (`mvp`/`strict`)
- readonly rootfs enabled
- CPU/memory/pids limits configured
- network mode status (`none` recommended)
- detached daemon log permission (`0600`)
- data directory safety checks (including WSL/NTFS pitfalls)

## Recommended Operational Practices

- Run daemon as root only where required; keep host patched.
- Keep `data_dir` on ext4/xfs (not NTFS in WSL).
- Prefer short session TTL and delete inactive sessions quickly.
- Use dedicated service account and minimal host access around Sandkasten.
- Review images before allowing them; avoid broad `allowed_images` lists.
- Monitor daemon logs and run `security` checks after config changes.

## Important Limitations

> [!CAUTION]
> No sandbox can provide an absolute guarantee against kernel-level breakout. Sandkasten uses layered defenses (namespaces, cgroups, capabilities drop, no-new-privs, seccomp, readonly rootfs), which significantly reduce risk—treat it as defense-in-depth, not a full security boundary.
