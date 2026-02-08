# Sandkasten Quickstart

**5-minute end-to-end example** — run a coding agent with a real Linux sandbox.

This quickstart includes:
- 🐳 Sandkasten daemon (Docker Compose)
- 🤖 Enhanced interactive agent (OpenAI Agents SDK)
- 🎨 Rich terminal UI with streaming & history
- 📦 Three example agents to try

## What You'll Get

```
┌─ Sandkasten Interactive Agent ─────────────────┐
│ Streaming • History • Rich UI                  │
└─────────────────────────────────────────────────┘

┌─ Session ───────────────────────────────────────┐
│ Sandbox:  abc123                                │
│ Image:    sandbox-runtime:python                │
│ Network:  full                                  │
└─────────────────────────────────────────────────┘

You: Write a Python script to calculate primes up to 100

  → exec: python3 --version
  → write: primes.py
  → exec: python3 primes.py

┌─ Assistant ─────────────────────────────────────┐
│ I've created and run a prime number calculator. │
│                                                  │
│ The script found all primes up to 100:          │
│ [2, 3, 5, 7, 11, 13, 17, 19, 23, ...]          │
└─────────────────────────────────────────────────┘
```

---

## Prerequisites

- Docker & Docker Compose
- Python 3.10+
- OpenAI API key

---

## Step 1: Build Sandbox Images

From the **repo root** (one directory up):

```bash
cd ..
task images
cd quickstart
```

This builds the base, Python, and Node sandbox images.

---

## Step 2: Start Sandkasten Daemon

```bash
cd daemon
docker-compose up -d
```

The daemon is now running at `http://localhost:8080` with API key `sk-sandbox-quickstart`.

**Check it's working:**

```bash
curl http://localhost:8080/healthz
# Should return: {"status":"ok"}
```

**View logs:**

```bash
docker-compose logs -f
```

---

## Step 3: Run the Agent

### Option A: Enhanced Interactive Agent (Recommended)

Beautiful terminal UI with streaming, history, and rich formatting:

```bash
cd ../agent

# Setup with uv
uv sync

# Set your OpenAI API key
export OPENAI_API_KEY="sk-..."

# Run the enhanced agent
uv run enhanced_agent.py
```

**Features:**
- 🎨 Rich terminal UI with boxes and colors
- ⚡ Streaming responses (token-by-token)
- 💾 Persistent conversation history (SQLite)
- 🔧 Visual tool execution feedback
- 📜 Commands: `/history`, `/clear`, `/help`, `/quit`

**Try it:**
```
You: Write a Python script that calculates prime numbers up to 100, save it as primes.py, and run it
```

### Option B: Simple Example

Single-task example (Fibonacci):

```bash
uv run coding_agent.py
```

### Option C: Basic Interactive

Simple interactive mode without fancy UI:

```bash
uv run interactive_agent.py
```

---

## Step 4: Try Example Tasks

In the enhanced agent, try these prompts:

**Code Generation:**
```
Write a Python script that scrapes the top HN stories and saves them to a JSON file
```

**Data Analysis:**
```
Create a CSV with random sales data (100 rows), then calculate monthly totals and plot a chart
```

**Web Development:**
```
Create a simple Flask API with /hello and /time endpoints, write tests, and run them
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

## Step 5: Clean Up

```bash
cd ../daemon
docker-compose down

# Remove volumes (optional)
docker-compose down -v
```

---

## Configuration

### Daemon Config (`daemon/sandkasten.yaml`)

```yaml
listen: "0.0.0.0:8080"
api_key: "sk-sandbox-quickstart"
session_ttl_seconds: 1800

defaults:
  cpu_limit: 1.0
  mem_limit_mb: 512
  network_mode: "none"  # Change to "full" if agent needs internet
```

### Agent Config (`agent/coding_agent.py`)

```python
SANDKASTEN_URL = "http://localhost:8080"
SANDKASTEN_API_KEY = "sk-sandbox-quickstart"
```

---

## Next Steps

- **Use a different image**: Change `"sandbox-runtime:python"` to `"sandbox-runtime:node"` in the agent
- **Persist workspace**: Add workspace persistence across sessions (see main README)
- **Add more tools**: Extend the agent with git, database, or API tools
- **Deploy to server**: Use the main `docker-compose.yml` with TLS and proper auth

---

## Troubleshooting

### "Connection refused"
The daemon isn't running. Check:
```bash
cd daemon
docker-compose ps
docker-compose logs
```

### "Image not found: sandbox-runtime:python"
You need to build images first:
```bash
cd ..
task images
```

### Agent hangs or times out
Check sandbox logs:
```bash
docker ps  # Find the sandkasten-<session_id> container
docker logs sandkasten-<session_id>
```

### "Invalid API key"
Make sure the API key matches in:
- `daemon/sandkasten.yaml` (`api_key`)
- `agent/coding_agent.py` (`SANDKASTEN_API_KEY`)

---

## Architecture

```
┌─────────────────────┐
│  coding_agent.py    │  ← Your Python script
│  (OpenAI Agents)    │
└──────────┬──────────┘
           │ HTTP (localhost:8080)
┌──────────▼──────────┐
│  Sandkasten Daemon  │  ← Docker Compose
│  (sandkasten.yaml)  │
└──────────┬──────────┘
           │ Docker API
┌──────────▼──────────────┐
│  Sandbox Container      │
│  (sandbox-runtime:python)│
│  - bash shell (stateful)│
│  - /workspace (volume)  │
└─────────────────────────┘
```

Each tool call from the agent → HTTP request → container exec → response.
