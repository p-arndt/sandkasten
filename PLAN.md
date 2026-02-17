# Sandkasten v2 (No Docker Runtime) — Architecture + Implementation Plan

Goal: a **stateful, “VM-like” sandbox** for agents (persistent shell + filesystem) that **scales** (no Docker volumes), runs on Linux and **Windows via WSL2**, and is simple to operate.

---

## 0) High-level concept

Replace “container per session” with **“process sandbox per session”**:

* **Base rootfs** (shared, read-only) per image (base/python/node)
* **Session layer** via **overlayfs** (upper/work dirs per session)
* **Workspace dir** bind-mounted into `/workspace` (optional persistence)
* Isolation via Linux primitives:

  * mount/pid/uts/ipc/user namespaces (+ optional netns)
  * cgroups v2 (cpu/mem/pids)
  * no_new_privs + drop caps + seccomp (MVP: minimal)
* Persistent shell maintained by the **runner** (PID 1 in the sandbox)

Windows support: run daemon in **WSL2** (WSL2 uses cgroup v2 now, which is good). ([Stack Overflow][1])

---

## 1) Scope / Non-goals (MVP)

### MVP includes

* Single-tenant API key auth
* Stateful sessions: `exec`, `read`, `write`
* Image store: base/python/node rootfs
* Overlay-backed session rootfs
* Workspace bind-mount
* TTL reaper + reconciliation
* TS SDK unchanged (same HTTP contract)

### Deferred

* Multi-tenant policies, per-tenant quotas
* Streaming exec output (SSE)
* Network egress proxy / allowlisting
* Snapshot/restore (optional Phase 2)
* Strong isolation via microVMs (optional driver later)

---

## 2) Directory layout (host)

All paths must live on a Linux filesystem that supports overlayfs/xattrs.
**On WSL2: keep data inside the Linux distro (ext4), not `/mnt/c` (NTFS), or overlay will bite you**. ([Stack Overflow][2])

```
/var/lib/sandkasten/
  images/
    <imageNameOrHash>/
      rootfs/                 # unpacked rootfs (read-only)
      meta.json
  sessions/
    <sessionId>/
      upper/                  # overlay upper
      work/                   # overlay workdir
      mnt/                    # overlay mountpoint (temporary)
      run/                    # bind-mounted into sandbox at /run/sandkasten
        runner.sock           # unix socket created by runner
      state.json              # cached runtime state (init pid, cgroup path, etc.)
  workspaces/
    <workspaceId>/
      ... user files ...
/run/sandkasten/
  daemon.sock (optional)
```

---

## 3) Components

### 3.1 Daemon (HTTP API + orchestration)

* Validates API key
* Creates sessions, tracks in SQLite
* Calls runtime driver to create/exec/destroy sessions
* Maintains per-session mutex for serialized shell exec
* TTL reaper + reconciliation on boot

### 3.2 Runtime Driver (Linux sandbox runtime)

Responsible for:

* Mount overlay rootfs
* Create namespaces + cgroups
* Pivot into new root
* Start runner as PID 1
* Provide daemon ↔ runner unix socket path
* Destroy session (kill cgroup, unmount, cleanup)

### 3.3 Runner (inside sandbox)

* PID 1 inside sandbox
* Creates PTY + starts `bash -l` (fallback `sh`)
* Listens on `/run/sandkasten/runner.sock` (host-visible via bind mount)
* Implements your JSON protocol (`exec`, `read`, `write`) with sentinel markers

### 3.4 Image Tooling

* Build rootfs tarballs for images (base/python/node)
* Unpack into `/var/lib/sandkasten/images/<image>/rootfs`
* Store metadata for validation/versioning

---

## 4) Project structure (recommended)

```
sandkasten/
├── cmd/
│   ├── sandkasten/              # daemon (linux)
│   ├── sandkasten-wsl/          # windows helper (optional)
│   ├── runner/                  # runner binary (linux)
│   └── imgbuilder/              # image build/import tool (linux)
├── internal/
│   ├── api/                     # http handlers
│   ├── auth/                    # api key middleware
│   ├── store/                   # sqlite + migrations
│   ├── session/                 # orchestration + locks
│   ├── reaper/                  # ttl + reconciliation
│   ├── runtime/
│   │   ├── driver.go            # interface (Create/Exec/Destroy/Inspect)
│   │   ├── linux/               # linux implementation
│   │   │   ├── create.go
│   │   │   ├── destroy.go
│   │   │   ├── exec.go          # connect to runner.sock
│   │   │   ├── nsinit.go        # stage1 reexec entrypoint
│   │   │   ├── mount.go         # overlay + bind mounts
│   │   │   ├── cgroup.go
│   │   │   ├── seccomp.go       # minimal mvp profile
│   │   │   └── util.go
│   ├── images/                  # image store (import/validate)
│   ├── config/                  # config loading
│   └── log/                     # logging helpers
├── protocol/                    # shared JSON types
├── sdk/                         # ts sdk (unchanged)
└── images/                      # rootfs recipes / build scripts (optional)
```

---

## 5) Data model (SQLite)

Keep your existing `sessions` table, but replace `container_id` with runtime fields:

```sql
CREATE TABLE sessions (
  id            TEXT PRIMARY KEY,
  image         TEXT NOT NULL,
  workspace_id  TEXT NOT NULL,
  init_pid      INTEGER NOT NULL,
  cgroup_path   TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'running',  -- running|expired|destroyed|crashed
  cwd           TEXT NOT NULL DEFAULT '/workspace',
  created_at    TEXT NOT NULL,
  expires_at    TEXT NOT NULL,
  last_activity TEXT NOT NULL
);
```

---

## 6) Runtime Driver details (Linux)

### 6.1 Preconditions (enforced at startup)

* Running on Linux kernel with:

  * overlayfs support
  * cgroups v2 mounted at `/sys/fs/cgroup`
  * ability to create namespaces and mounts (daemon typically runs as root)
* On WSL2: ensure data dir is inside distro ext4; WSL2 uses cgroup v2. ([Stack Overflow][1])

Startup checks:

* `statfs(/sys/fs/cgroup)` and detect v2
* verify overlayfs mount works via a tiny probe
* ensure mount propagation can be set private

### 6.2 Filesystem setup (per session)

Given:

* `lower = /var/lib/sandkasten/images/<image>/rootfs`
* `upper = /var/lib/sandkasten/sessions/<id>/upper`
* `work  = /var/lib/sandkasten/sessions/<id>/work`
* `mnt   = /var/lib/sandkasten/sessions/<id>/mnt`
* `runHostDir = /var/lib/sandkasten/sessions/<id>/run`
* `workspace = /var/lib/sandkasten/workspaces/<workspaceId>`

Steps:

1. `mkdir -p upper work mnt runHostDir workspace`
2. `mount("overlay", mnt, "overlay", 0, "lowerdir=...,upperdir=...,workdir=...")`
3. bind-mount workspace into sandbox:

   * create `<mnt>/workspace` if missing
   * `mount(workspace, <mnt>/workspace, "", MS_BIND|MS_REC, "")`
4. bind-mount host run dir into sandbox:

   * create `<mnt>/run/sandkasten`
   * `mount(runHostDir, <mnt>/run/sandkasten, "", MS_BIND, "")`
5. mount `tmpfs` for `<mnt>/tmp`
6. minimal `/dev`:

   * create `<mnt>/dev`, mount `tmpfs` there, create needed nodes or bind-mount safe ones (`/dev/null`, `/dev/zero`, `/dev/random`, `/dev/urandom`, `/dev/tty`, `/dev/ptmx`)
7. do **not** mount host `/sys` into sandbox (keep it out)

### 6.3 Namespace + pivot_root (stage1 “nsinit”)

Go needs a “stage1” path (like runc). Use **reexec**:

* Daemon binary launches itself with env `SANDKASTEN_STAGE=nsinit`
* `SysProcAttr.Cloneflags` creates namespaces:

  * `CLONE_NEWNS | CLONE_NEWPID | CLONE_NEWUTS | CLONE_NEWIPC`
  * optional: `CLONE_NEWNET` (MVP can skip, or always create with no interfaces)
  * optional: `CLONE_NEWUSER` (nice-to-have; often painful—can defer and run rootful inside ns)
* The nsinit process:

  1. `mount("", "/", "", MS_REC|MS_PRIVATE, "")` (stop propagation)
  2. `sethostname("sk-"+id)`
  3. `chdir(mnt)`
  4. `pivot_root(mnt, mnt+"/.oldroot")`
  5. `umount2("/.oldroot", MNT_DETACH)` and remove
  6. mount `proc` at `/proc`
  7. drop privileges to `sandbox` user (e.g. uid=1000 gid=1000) **after** mounts
  8. apply hardening:

     * set `PR_SET_NO_NEW_PRIVS`
     * drop capabilities (bounding set)
     * apply seccomp filter (MVP: minimal denylist)
  9. exec `/usr/local/bin/runner`

**Important:** In a new PID namespace, nsinit becomes PID 1 briefly, then execs runner. Runner becomes PID 1.

### 6.4 cgroups v2 (per session)

Create:
`/sys/fs/cgroup/sandkasten/<id>/`

Set limits (examples):

* `memory.max = <memBytes>`
* `pids.max = <pids>`
* `cpu.max = "<quota> <period>"` (e.g. `100000 100000` for 1 CPU)

Attach the sandbox init pid:

* write PID into `cgroup.procs`

On destroy:

* kill processes in cgroup (see below), then remove directory.

### 6.5 Killing / cleanup (reliable destroy)

Destroy steps:

1. mark session status `destroying`
2. send `SIGTERM` to init pid (host pid)
3. wait `grace_ms` (e.g. 500ms–2s)
4. if still alive: `SIGKILL`
5. additionally ensure cgroup empty:

   * read `cgroup.procs`; SIGKILL any remaining
6. `umount2(mnt, MNT_DETACH)`
7. remove `upper/work/mnt/run` dirs
8. update DB: `destroyed`

### 6.6 Runner socket connectivity

Because `/run/sandkasten` is bind-mounted from host session dir, runner creates:
`/run/sandkasten/runner.sock`

Daemon connects at host path:
`/var/lib/sandkasten/sessions/<id>/run/runner.sock`

Exec flow:

* daemon opens unix socket
* writes one JSON request line
* reads one JSON response line
* closes

This keeps daemon restart-safe (no long-lived connections needed).

---

## 7) Runner (inside sandbox) — MVP spec

Keep your protocol and sentinel approach.

### Runner startup

* create unix socket listener at `/run/sandkasten/runner.sock`
* create PTY
* spawn `bash -l` (fallback `sh`)
* keep PTY alive for session lifetime

### Request handling

* accept one connection → read one JSON line → process → write one JSON line → close
* `exec`: serialize with internal mutex (PTY is shared)
* `read/write`: operate on filesystem (enforce path under `/workspace`)

### Path safety

* Clean + validate:

  * no `..` traversal
  * must be inside `/workspace` (realpath check)

### Output limits

* cap output bytes (e.g. 2–8MB) and set `truncated=true`

### Timeouts

* exec timeout: enforce by timer + kill process group in PTY (send ctrl-c then kill if needed)

---

## 8) HTTP API (same as your plan)

Keep endpoints:

```
POST   /v1/sessions
GET    /v1/sessions/{id}
POST   /v1/sessions/{id}/exec
POST   /v1/sessions/{id}/fs/write
GET    /v1/sessions/{id}/fs/read
DELETE /v1/sessions/{id}
GET    /v1/sessions
```

Request/response payloads unchanged.

---

## 9) Image format + tooling

### 9.1 What is an “image” now?

An image is a **rootfs directory** with:

* `/usr/local/bin/runner`
* shell + coreutils + certs
* optional language runtimes (python/node)

### 9.2 How to build images (two acceptable methods)

**Method A (simple, uses Docker at build time only)**

* Build a container image from Dockerfile
* Export rootfs:

  * `docker create ...`
  * `docker export` → tar
* Import tar into sandkasten image store and unpack

**Method B (pure Linux, no Docker anywhere)**

* Use `debootstrap`/`mmdebstrap` to create rootfs tar
* `chroot` install packages
* Copy runner into `/usr/local/bin/runner`

MVP recommendation: Method A first (build-time only). Runtime stays Docker-free.

### 9.3 Image import command (`imgbuilder`)

* `sandkasten-img import --name python --tar python-rootfs.tar`
* compute hash, unpack to `/var/lib/sandkasten/images/<hash>/rootfs`
* write `meta.json`:

  * name, hash, created_at
  * packages/runtime versions (optional)
  * runner version

---

## 10) Windows / WSL2 packaging

### 10.1 Supported mode

* Daemon runs inside WSL2 distro.
* TS SDK talks to `http://127.0.0.1:<port>` (from Windows it can access WSL localhost).

WSL2 config is controlled via `wsl.conf` / `.wslconfig`. ([Microsoft Learn][3])

### 10.2 Hard requirement

* Store `/var/lib/sandkasten` inside WSL ext4 (default).
* Do **not** place it under `/mnt/c/...` due to overlay/xattr constraints. ([Stack Overflow][2])

### 10.3 Optional helper (`sandkasten-wsl.exe`)

* Ensures WSL installed + distro present
* Copies linux binary into distro or downloads release
* Starts daemon via `wsl.exe -d <distro> -- sudo sandkasten ...`
* Shows status + logs

---

## 11) Security posture (MVP vs later)

### MVP hardening (do these)

* `no_new_privs`
* drop all capabilities
* readonly base rootfs (overlay lower is ro)
* tmpfs `/tmp`
* do not mount host `/sys`
* pids + memory limits via cgroup v2
* enforce workspace path constraints
* default: **no network** (skip netns or create netns with nothing configured)

### Next hardening steps (Phase 2)

* seccomp allowlist (tight)
* landlock policy for filesystem
* egress proxy-only networking
* per-session UID mapping (userns) if you want “rootless in sandbox”
* optional Firecracker driver for “high risk” sessions

---

## 12) Reaper + reconciliation

### TTL reaper

Every 30s:

* query sessions where `expires_at < now AND status='running'`
* destroy runtime
* set status `expired`

### Reconciliation on startup

* scan `/var/lib/sandkasten/sessions/*/state.json`
* for each:

  * if DB missing or status not running → destroy + cleanup dir
  * if DB says running but init pid dead → mark `crashed` + cleanup

Store minimal runtime state in `state.json`:

```json
{
  "session_id": "...",
  "init_pid": 12345,
  "cgroup_path": "/sys/fs/cgroup/sandkasten/<id>",
  "mnt": "/var/lib/sandkasten/sessions/<id>/mnt",
  "runner_sock": "/var/lib/sandkasten/sessions/<id>/run/runner.sock"
}
```

---

## 13) Config (suggested)

```yaml
listen: "127.0.0.1:8080"
api_key: "sk-sandbox-..."

data_dir: "/var/lib/sandkasten"

default_image: "python"
allowed_images: ["base","python","node"]

session_ttl_seconds: 1800

limits:
  cpu_quota: 1.0
  mem_limit_mb: 512
  pids_limit: 256
  max_exec_timeout_ms: 120000

network:
  mode: "none"   # none | netns (later) | proxy (later)

security:
  seccomp: "mvp" # off | mvp | strict (later)
```

---

## 14) Implementation order (agent-friendly checklist)

### Phase 1 — Foundations

1. **protocol/**

   * define request/response structs (exec/read/write/error)
2. **runner/**

   * PTY + shell + sentinel markers
   * unix socket server at `/run/sandkasten/runner.sock`
   * path sandboxing to `/workspace`
3. **imgbuilder/**

   * import rootfs tar → image store unpack + meta.json
   * list images, validate required files exist

### Phase 2 — Linux runtime driver

4. **runtime/linux/mount.go**

   * overlay mount + bind mounts + tmpfs + minimal /dev
   * unmount cleanup
5. **runtime/linux/cgroup.go**

   * create cgroup dir
   * write limits
   * attach pid
   * kill all procs on destroy
6. **runtime/linux/nsinit.go**

   * reexec stage1
   * pivot_root + proc mount
   * drop privs, no_new_privs, caps drop
   * seccomp MVP (can start “off”, add later)
7. **runtime/linux/create.go**

   * create dirs
   * mount
   * create cgroup
   * start nsinit → runner
   * wait until runner socket exists (with timeout)
   * write state.json
8. **runtime/linux/exec.go**

   * connect to runner.sock, send request, read response
9. **runtime/linux/destroy.go**

   * SIGTERM/SIGKILL init pid
   * cgroup kill fallback
   * unmount + delete dirs

### Phase 3 — Daemon API + store

10. **store/**

    * migrations + CRUD for sessions
11. **session manager**

    * create/exec/read/write/destroy
    * per-session mutex map
    * update `last_activity`, extend `expires_at`
12. **api/**

    * routes + middleware + handlers
13. **reaper/**

    * ttl sweep + reconciliation on boot
14. **integration tests**

    * create session → `exec "pwd"` → write file → read file → install pip pkg → run python → destroy
    * verify memory/pids limits behave
    * verify no-network mode (if enabled) blocks `curl`/DNS

### Phase 4 — WSL packaging (optional)

15. `sandkasten-wsl.exe` helper or docs + script to run daemon inside WSL

---

## 15) Acceptance criteria (MVP)

* Can run 100+ concurrent sessions without Docker
* Session create < ~100ms (after image unpack)
* Exec is stateful (`cd` persists)
* TTL cleanup leaves no mounts behind (`mount | grep sandkasten` empty)
* After daemon crash/restart, reconciliation cleans orphan sessions
* Works in WSL2 **when data dir is inside Linux filesystem** ([Stack Overflow][2])

---

## 16) Optional Phase 2 upgrades (once MVP is stable)

* Workspace snapshots (`tar.zst`) + restore
* Output streaming over SSE
* Warm session pools per image
* Proxy-only egress networking
* Strict seccomp + landlock
* Alternative driver: Firecracker for “high-risk” runs (same API)

---

[1]: https://stackoverflow.com/questions/79469388/how-to-enable-cgroup-v1-in-wsl2?utm_source=chatgpt.com "How to enable cgroup v1 in WSL2"
[2]: https://stackoverflow.com/questions/76481801/error-after-moving-dockers-dir-to-ntfs-overlayfs-upper-fs-does-not-support-x?utm_source=chatgpt.com "Error after moving Docker's dir to NTFS: overlayfs: upper fs ..."
[3]: https://learn.microsoft.com/en-us/windows/wsl/wsl-config?utm_source=chatgpt.com "Advanced settings configuration in WSL"

