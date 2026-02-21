# Windows Setup Guide (WSL2)

Sandkasten requires Linux kernel features (namespaces, cgroups v2, overlayfs). On Windows, use WSL2.

## Prerequisites

- Windows 10 (version 2004+) or Windows 11
- WSL2 enabled
- Ubuntu 22.04+ (or any Linux distro with cgroups v2)

## Step 1: Install WSL2

Open PowerShell as Administrator:

```powershell
# Enable WSL
wsl --install

# Or install specific distro
wsl --install -d Ubuntu-22.04
```

Restart if prompted, then complete Ubuntu setup (username, password).

## Step 2: Install Dependencies in WSL2

Open WSL2:

```powershell
wsl
```

Inside WSL2:

```bash
# Update packages
sudo apt update && sudo apt upgrade -y

# Install Go 1.24+
sudo apt install -y golang-go git curl

# Verify Go version (need 1.24+)
go version

# If Go is too old, install manually:
# wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
# sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
# echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
# source ~/.bashrc
```

Install Task (build tool):

```bash
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin
echo 'export PATH=$PATH:$HOME/.local/bin' >> ~/.bashrc
source ~/.bashrc
```

## Step 3: Clone and Build

> [!IMPORTANT]
> Clone inside the WSL filesystem (e.g. `~/sandkasten`), not on `/mnt/c/`. Building on NTFS can cause issues.

```bash
cd ~
git clone https://github.com/p-arndt/sandkasten
cd sandkasten
task build
```

This creates:
- `bin/runner` - Runner binary (PID 1 in sandboxes)
- `bin/sandkasten` - The daemon
- `bin/imgbuilder` - Image management tool

## Step 4: Create an Image

You need at least one rootfs image. Preferred: pull from a registry (no Docker daemon required):

```bash
# Pull from registry (recommended)
sudo ./bin/sandkasten image pull --name python python:3.12-slim
./bin/sandkasten image list
```

Alternatively, export from Docker and import with imgbuilder:

```bash
docker create --name temp python:3.12-slim
docker export temp | gzip > /tmp/python.tar.gz
docker rm temp
sudo ./bin/imgbuilder import --name python --tar /tmp/python.tar.gz
```

## Step 5: Create Configuration

```bash
cat > sandkasten.yaml << 'EOF'
listen: "127.0.0.1:8080"
api_key: "sk-sandbox-test"
data_dir: "/var/lib/sandkasten"
default_image: "python"

defaults:
  cpu_limit: 1.0
  mem_limit_mb: 512
  pids_limit: 256
  network_mode: "none"
EOF
```

## Step 6: Create Data Directories

> [!IMPORTANT]
> Use a path on the **Linux filesystem** (e.g. `/var/lib/sandkasten`). Do not use `/mnt/c/...`—NTFS does not support overlayfs.

```bash
sudo mkdir -p /var/lib/sandkasten/{images,sessions,workspaces}
```

## Step 7: Start the Daemon

You can start the daemon either **inside WSL** (see below) or from **Windows** using the **sandkasten-wsl** helper.

### Option A: From inside WSL

```bash
# Run with sudo (required for namespaces)
# Foreground (logs in terminal):
sudo ./bin/sandkasten --config sandkasten.yaml

# Or in background (like Docker daemon):
sudo ./bin/sandkasten daemon -d --config sandkasten.yaml
```

### Option B: From Windows (sandkasten-wsl helper)

The **sandkasten-wsl** executable lets you start, check, and stop the daemon from Windows PowerShell without opening a WSL shell. Build it (from any OS) with:

```bash
task sandkasten-wsl
# or: GOOS=windows go build -o bin/sandkasten-wsl.exe ./cmd/sandkasten-wsl
```

Then on Windows, ensure the Linux binary and config exist inside WSL (e.g. you built and configured Sandkasten in WSL as in Steps 3–6). From PowerShell:

```powershell
# Start daemon in default WSL distro
.\bin\sandkasten-wsl.exe start

# Use a specific distro and config
.\bin\sandkasten-wsl.exe start --distro Ubuntu-22.04 --config ~/sandkasten.yaml

# Check if daemon is running
.\bin\sandkasten-wsl.exe status

# Stop daemon
.\bin\sandkasten-wsl.exe stop
```

The helper runs `wsl -d <distro> -- sudo sandkasten daemon -d ...` for you. The `sandkasten` binary must be on the PATH inside that WSL distro, or pass `--binary /path/to/sandkasten`.

---

To list sessions from WSL: `./bin/sandkasten ps`

You should see (when running in foreground):

```
INFO listening addr=127.0.0.1:8080

  sandkasten daemon ready
  API:       http://127.0.0.1:8080/v1
```

## Step 8: Test from WSL2

Open a new terminal and run:

```bash
# Check health
curl http://localhost:8080/healthz
# {"status":"ok"}

# Create a session
curl -X POST http://localhost:8080/v1/sessions \
  -H "Authorization: Bearer sk-sandbox-test" \
  -H "Content-Type: application/json" \
  -d '{"image":"python"}'
# {"id":"abc123def456","image":"python","status":"running",...}

# Save the session ID
SESSION_ID="abc123def456"  # Use the ID from above
```

### Execute Commands

```bash
# Simple command
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/exec \
  -H "Authorization: Bearer sk-sandbox-test" \
  -H "Content-Type: application/json" \
  -d '{"cmd":"echo hello world"}'

# Response:
# {"exit_code":0,"cwd":"/workspace","output":"hello world\n","truncated":false,"duration_ms":42}
```

```bash
# Run Python
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/exec \
  -H "Authorization: Bearer sk-sandbox-test" \
  -H "Content-Type: application/json" \
  -d '{"cmd":"python3 -c \"print(2 + 2)\""}'

# Response:
# {"exit_code":0,"cwd":"/workspace","output":"4\n",...}
```

```bash
# Install package and use it
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/exec \
  -H "Authorization: Bearer sk-sandbox-test" \
  -H "Content-Type: application/json" \
  -d '{"cmd":"pip install requests -q && python3 -c \"import requests; print(requests.__version__)\""}'
```

### Write and Read Files

```bash
# Write a file
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/fs/write \
  -H "Authorization: Bearer sk-sandbox-test" \
  -H "Content-Type: application/json" \
  -d '{"path":"/workspace/hello.py","text":"print(\"Hello from file!\")"}'

# Read it back
curl "http://localhost:8080/v1/sessions/$SESSION_ID/fs/read?path=/workspace/hello.py" \
  -H "Authorization: Bearer sk-sandbox-test"

# Run it
curl -X POST http://localhost:8080/v1/sessions/$SESSION_ID/exec \
  -H "Authorization: Bearer sk-sandbox-test" \
  -H "Content-Type: application/json" \
  -d '{"cmd":"python3 /workspace/hello.py"}'
```

### Cleanup

```bash
# Destroy session
curl -X DELETE http://localhost:8080/v1/sessions/$SESSION_ID \
  -H "Authorization: Bearer sk-sandbox-test"
```

## Step 9: Access from Windows

The daemon running in WSL2 is accessible from Windows via `localhost`:

### PowerShell

```powershell
# Check health
curl http://localhost:8080/healthz

# Create session
$session = Invoke-RestMethod -Uri "http://localhost:8080/v1/sessions" `
  -Method POST `
  -Headers @{ "Authorization" = "Bearer sk-sandbox-test" } `
  -ContentType "application/json" `
  -Body '{"image":"python"}'

$sessionId = $session.id
Write-Host "Session ID: $sessionId"

# Execute command
$result = Invoke-RestMethod -Uri "http://localhost:8080/v1/sessions/$sessionId/exec" `
  -Method POST `
  -Headers @{ "Authorization" = "Bearer sk-sandbox-test" } `
  -ContentType "application/json" `
  -Body '{"cmd":"python3 -c \"print(42)\""}'

Write-Host "Output: $($result.output)"
```

### Python (Windows)

```powershell
pip install sandkasten
```

```python
import asyncio
from sandkasten import SandboxClient

async def main():
    client = SandboxClient(
        base_url="http://localhost:8080",
        api_key="sk-sandbox-test"
    )
    
    async with await client.create_session(image="python") as session:
        # Execute command
        result = await session.exec("python3 -c 'print(2 + 2)'")
        print(result.output)  # 4
        
        # Write and run a file
        await session.write("test.py", "print('Hello!')")
        result = await session.exec("python3 test.py")
        print(result.output)  # Hello!

asyncio.run(main())
```

## Important: Filesystem Location

**Always work inside WSL's Linux filesystem** (`/home/...`), NOT the Windows mount (`/mnt/c/...`):

```bash
# ✅ GOOD - Linux filesystem (ext4, supports overlayfs)
cd ~/sandkasten
sudo ./bin/sandkasten

# ❌ BAD - Windows mount (NTFS, no overlayfs support)
cd /mnt/c/Users/You/sandkasten
sudo ./bin/sandkasten  # Will fail!
```

If you need to edit code from Windows, use an IDE that supports WSL2:
- VS Code: Install "WSL" extension, then `code .` from WSL
- JetBrains IDEs: Use remote development via SSH to localhost

## Troubleshooting

### "cgroup v2 not mounted" / cgroup errors

Sandkasten needs **unified cgroups (cgroups v2)** so it can isolate and limit sandbox processes. By default, WSL2 may boot without them, so the daemon reports a cgroup error and refuses to start.

**Option A: Enable unified cgroups in WSL2 (recommended)**

You don’t replace your existing `.wslconfig` — you **add** kernel boot flags.

1. **Edit** `C:\Users\<YourUsername>\.wslconfig` (in your Windows user profile, not inside WSL).

   If the file already has `[wsl2]` and e.g. `nestedVirtualization=true`, add the `kernelCommandLine` line. Full example:

   ```ini
   [wsl2]
   nestedVirtualization=true
   kernelCommandLine=systemd.unified_cgroup_hierarchy=1 cgroup_no_v1=all
   ```

   Important:
   - No spaces before `[wsl2]`
   - `kernelCommandLine` is one line, no quotes
   - Save the file

2. **Fully reboot WSL** (this step is often done wrong):
   - Close **all** WSL terminals.
   - In **PowerShell** (on Windows):

     ```powershell
     wsl --shutdown
     ```
   - Wait **5–10 seconds** so the WSL VM actually stops.
   - Start WSL again:

     ```powershell
     wsl
     ```

3. **Verify** inside WSL:

   ```bash
   stat -fc %T /sys/fs/cgroup
   ```

   You want the output:

   ```
   cgroup2fs
   ```

   If you see `cgroup2fs`, the runtime can start. Try:

   ```bash
   sudo ./bin/sandkasten --config sandkasten.yaml
   ```

   The cgroup error should be gone.

**Why this happens:** The program checks whether it’s on a Linux system where it can safely isolate processes. Without unified cgroups, WSL behaves like an older compatibility environment. After this change, WSL uses a proper systemd-managed Linux VM with cgroups v2, which sandkasten (and other container runtimes) require.

**Option B: Ensure WSL2 (not WSL1)**

If you still have issues, confirm you’re on WSL2:

```powershell
# In PowerShell
wsl --list --verbose

# Should show:
# NAME            VERSION
# Ubuntu-22.04    2
```

If VERSION is 1, upgrade:

```powershell
wsl --set-version Ubuntu-22.04 2
```

### "overlayfs: upper fs does not support xattrs"

You're trying to run from `/mnt/c/...` (NTFS). Move to Linux filesystem:

```bash
# Move project to home directory
cp -r /mnt/c/Users/You/sandkasten ~/sandkasten
cd ~/sandkasten
```

Also ensure `data_dir` is on Linux filesystem:

```yaml
# ✅ Correct
data_dir: "/var/lib/sandkasten"

# ❌ Wrong
data_dir: "/mnt/c/sandkasten"
```

### "permission denied" / namespace errors

The daemon needs root for namespace operations:

```bash
sudo ./bin/sandkasten --config sandkasten.yaml
```

### "image not found"

Import at least one image:

```bash
./bin/imgbuilder list
# If empty:
sudo ./bin/imgbuilder import --name python --tar /tmp/python.tar.gz
```

### Connection refused from Windows

1. Check daemon is running in WSL2
2. Check WSL2 port forwarding:

```powershell
# In PowerShell
netstat -an | findstr 8080
```

3. If still not working, find WSL2 IP and use directly:

```bash
# In WSL2
ip addr show eth0 | grep 'inet ' | awk '{print $2}' | cut -d/ -f1
# e.g., 172.20.10.2
```

Then access from Windows: `http://172.20.10.2:8080`

### Go version too old

If `go version` shows < 1.24:

```bash
# Remove old version
sudo apt remove golang-go

# Install manually
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify
go version
```

## Quick Reference

```bash
# Build
task build

# Import image
docker create --name temp python:3.12-slim
docker export temp | gzip > /tmp/python.tar.gz && docker rm temp
sudo ./bin/imgbuilder import --name python --tar /tmp/python.tar.gz

# Start daemon
sudo ./bin/sandkasten --config sandkasten.yaml

# Test
curl http://localhost:8080/healthz
```

## API Quick Reference

```bash
API_KEY="sk-sandbox-test"
BASE="http://localhost:8080"

# Create session
SESSION=$(curl -s -X POST $BASE/v1/sessions \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"image":"python"}' | jq -r .id)

# Execute command
curl -s -X POST $BASE/v1/sessions/$SESSION/exec \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"cmd":"echo hello"}' | jq .

# Write file
curl -s -X POST $BASE/v1/sessions/$SESSION/fs/write \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"path":"/workspace/test.py","text":"print(42)"}' | jq .

# Read file
curl -s "$BASE/v1/sessions/$SESSION/fs/read?path=/workspace/test.py" \
  -H "Authorization: Bearer $API_KEY" | jq -r .content_base64 | base64 -d

# Destroy session
curl -s -X DELETE $BASE/v1/sessions/$SESSION \
  -H "Authorization: Bearer $API_KEY"
```
