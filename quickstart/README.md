# Sandkasten Quickstart

**1-minute setup** — run a coding agent with a real Linux sandbox.

This quickstart includes:
- Sandkasten daemon (native Linux sandboxing)
- Enhanced interactive agent (OpenAI Agents SDK)
- Rich terminal UI with streaming & history
- Three example agents to try

## Prerequisites

- Linux (kernel 5.11+) or WSL2
- Python 3.10+ (for the agent)
- OpenAI API key

---

## Step 1: Start Sandkasten

**Option A: Docker (no build needed)**

```bash
docker run -d --privileged --name sandkasten \
  -p 8080:8080 \
  -v sandkasten-data:/var/lib/sandkasten \
  -e SANDKASTEN_API_KEY=sk-sandbox-quickstart \
  ghcr.io/p-arndt/sandkasten:latest \
  /bin/sandkasten up
```

**Option B: Install binary**

```bash
curl -fsSL https://raw.githubusercontent.com/p-arndt/sandkasten/main/scripts/install.sh | sudo bash
sudo sandkasten up
```

**Option C: Build from source**

```bash
# From repo root
task build
sudo ./bin/sandkasten up
```

`sandkasten up` handles everything: environment checks, data directories, image pulling, API key generation, and daemon startup.

Check it's working:

```bash
curl http://localhost:8080/healthz   # Should return: {"status":"ok"}
```

---

## Step 2: Run the Agent

### Enhanced Interactive Agent (recommended)

Beautiful terminal UI with streaming, history, and rich formatting:

```bash
cd quickstart/agent

# Setup with uv
uv sync

# Set your OpenAI API key
export OPENAI_API_KEY="sk-..."

# Run the enhanced agent
uv run enhanced_agent.py
```

**Features:**
- Rich terminal UI with boxes and colors
- Streaming responses (token-by-token)
- Persistent conversation history (SQLite)
- Visual tool execution feedback
- Commands: `/history`, `/clear`, `/help`, `/quit`

**Try it:**
```
You: Write a Python script that calculates prime numbers up to 100, save it as primes.py, and run it
```

### Simple Example

Single-task example (Fibonacci):

```bash
uv run coding_agent.py
```

### Basic Interactive

Simple interactive mode without fancy UI:

```bash
uv run interactive_agent.py
```

---

## Step 3: Try Example Tasks

In the enhanced agent, try these prompts:

**Code Generation:**
```
Write a Python script that generates Fibonacci numbers and saves them to a file
```

**Data Analysis:**
```
Create a CSV with random sales data (100 rows), then calculate monthly totals
```

**System Tasks:**
```
Check the system info (OS, CPU, memory), create a report file with the results
```

**Commands:**
- `/history` — Show conversation history
- `/clear` — Clear conversation history
- `/help` — Show help
- `/quit` — Exit

---

## Configuration

### Daemon Config (`sandkasten.yaml`)

```yaml
listen: "127.0.0.1:8080"
api_key: "sk-sandbox-quickstart"
data_dir: "/var/lib/sandkasten"
default_image: "python"

defaults:
  cpu_limit: 1.0
  mem_limit_mb: 512
  network_mode: "none"
```

### Agent Config

The agent reads from environment:

```bash
export SANDKASTEN_URL="http://localhost:8080"
export SANDKASTEN_API_KEY="sk-sandbox-quickstart"
```

---

## Next Steps

- **Add more images**: Import Node.js, custom images
- **Persist workspace**: Use `workspace_id` to persist files across sessions
- **Add more tools**: Extend the agent with git, database, or API tools

---

## Troubleshooting

### "Connection refused"
The daemon isn't running:
```bash
sudo ./bin/sandkasten --config sandkasten.yaml
```

### "Image not found: python"
Import the image:
```bash
sudo ./bin/imgbuilder import --name python --tar python.tar.gz
```

### "cgroup v2 not mounted"
Ensure you're on Linux with cgroups v2:
```bash
mount | grep cgroup2
```

### "overlayfs: upper fs does not support xattrs"
On WSL2, store data in Linux filesystem:
```yaml
data_dir: "/var/lib/sandkasten"  # Correct
# data_dir: "/mnt/c/..."         # Wrong (NTFS)
```

### "permission denied"
Daemon needs root for namespace operations:
```bash
sudo ./bin/sandkasten --config sandkasten.yaml
```

---

## Architecture

```
┌─────────────────────┐
│  enhanced_agent.py  │  ← Your Python script
│  (OpenAI Agents)    │
└──────────┬──────────┘
           │ HTTP (localhost:8080)
┌──────────▼──────────┐
│  Sandkasten Daemon  │  ← Native Linux sandboxing
│  (sandkasten.yaml)  │
└──────────┬──────────┘
           │ Namespaces + cgroups + overlayfs
┌──────────▼──────────────┐
│  Sandbox Process        │
│  - Runner (PID 1)       │
│  - bash shell (stateful)│
│  - /workspace (bind)    │
└─────────────────────────┘
```

Each tool call from the agent → HTTP request → sandbox exec → response.
