# Sandkasten Agent Examples

Three example agents demonstrating different use cases.

## 1. Enhanced Interactive Agent (Recommended)

**File:** `enhanced_agent.py`

The full-featured interactive agent with:

### Features
- 🎨 **Rich Terminal UI** — Boxes, colors, formatted output using [Rich](https://rich.readthedocs.io/)
- ⚡ **Streaming Responses** — Token-by-token streaming using `Runner.run_streamed()`
- 💾 **Persistent History** — SQLite-backed conversation history across sessions
- 🔧 **Tool Feedback** — Visual indicators for exec, write, read operations
- 📜 **Commands** — `/history`, `/clear`, `/help`, `/quit`

### Usage

```bash
export OPENAI_API_KEY="sk-..."
uv run enhanced_agent.py
```

### Example Session

```
┌─ Sandkasten Interactive Agent ─────────────────┐
│ Streaming • History • Rich UI                  │
└─────────────────────────────────────────────────┘

You: Create a Python script that generates QR codes

  → exec: pip install qrcode pillow
  → write: qr_gen.py
  → exec: python3 qr_gen.py

┌─ Assistant ─────────────────────────────────────┐
│ I've created a QR code generator script and    │
│ tested it. It successfully generates a QR code  │
│ for "https://example.com" as qr.png            │
└─────────────────────────────────────────────────┘
```

### Commands

- `/history` — Display all messages in the current conversation
- `/clear` — Clear conversation history (start fresh)
- `/help` — Show available commands
- `/quit` or `/exit` — Exit the agent

### Conversation History

History is stored in `conversation_history.db` using SQLite. Each session gets a unique ID with timestamp. You can resume conversations by using the same session ID.

---

## 2. Simple Task Agent

**File:** `coding_agent.py`

Single-task example that demonstrates basic agent usage.

### What It Does
1. Creates a sandbox session
2. Runs a predefined task (Fibonacci script)
3. Shows the agent's execution steps
4. Destroys the session

### Usage

```bash
uv run coding_agent.py
```

### Customization

Edit the `main()` function to change the task:

```python
task = (
    "Write a Python script that fetches weather data from wttr.in "
    "and displays it in a nice format"
)
```

---

## 3. Basic Interactive Agent

**File:** `interactive_agent.py`

Simple interactive mode without Rich UI — just plain text.

### Features
- Command-line input/output
- Tool execution feedback
- No external UI dependencies

### Usage

```bash
uv run interactive_agent.py
```

---

## Configuration

### Sandkasten Connection

All agents connect to the daemon via:

```python
SANDKASTEN_URL = "http://localhost:8080"
SANDKASTEN_API_KEY = "sk-sandbox-quickstart"
```

Update these if you've changed the daemon configuration.

### Sandbox Image

Default image is `sandbox-runtime:python`. Change it in the `create_session()` call:

```python
session_id = await create_session(image="sandbox-runtime:node")
```

Available images:
- `sandbox-runtime:base` — Minimal (bash, coreutils)
- `sandbox-runtime:python` — Python 3 + pip
- `sandbox-runtime:node` — Node.js 22 + npm

### Agent Instructions

Customize the agent's behavior by editing the `instructions` field:

```python
agent = Agent(
    name="data-analyst",
    instructions="""You are a data analysis expert.

    Focus on:
    - Clean, efficient pandas code
    - Data visualization with matplotlib
    - Statistical analysis
    - Clear explanations of findings
    """,
    tools=[exec, write_file, read_file],
)
```

---

## Tools

All agents have access to three tools:

### `exec(cmd: str, timeout_ms: int = 30000) -> str`

Execute a shell command in the sandbox.

**Stateful:** Directory changes, environment variables, and background processes persist.

```python
await exec("cd /workspace && mkdir data")
await exec("export API_KEY=xyz")
await exec("python server.py &")  # runs in background
```

### `write_file(path: str, content: str) -> str`

Write text to a file.

```python
await write_file("hello.py", "print('hello world')")
```

### `read_file(path: str) -> str`

Read a file's contents.

```python
content = await read_file("output.txt")
```

---

## Dependencies

Managed via `pyproject.toml` and `uv`:

```toml
dependencies = [
    "openai-agents>=0.8.1",
    "httpx>=0.28.1",
    "rich>=13.0.0",
]
```

Install/update:

```bash
uv sync
```

---

## Tips

### Streaming vs Non-Streaming

- **Streaming** (`Runner.run_streamed()`) — Token-by-token updates, better UX
- **Non-streaming** (`Runner.run()`) — Simpler, returns final result only

### History Management

SQLiteSession automatically saves and loads conversation context:

```python
session = SQLiteSession("user_123", "history.db")

# First turn
await Runner.run(agent, "Hello", session=session)

# Second turn - context preserved
await Runner.run(agent, "What did I just say?", session=session)
```

### Tool Execution Visibility

The enhanced agent shows tool calls in real-time:

```
  → exec: ls -la
  → write: data.json
  → read: results.txt
```

This helps debug and understand agent behavior.

### Error Handling

All HTTP errors are propagated. Wrap tool calls in try/except if needed:

```python
@function_tool
async def safe_exec(cmd: str) -> str:
    try:
        return await exec(cmd)
    except httpx.HTTPError as e:
        return f"Error: {e}"
```

---

## Troubleshooting

### "Connection refused"
Daemon isn't running. Start it:
```bash
cd ../daemon && docker-compose up -d
```

### "Invalid API key"
API key mismatch. Check:
- `../daemon/sandkasten.yaml` (`api_key: "sk-sandbox-quickstart"`)
- Agent files (`SANDKASTEN_API_KEY = "sk-sandbox-quickstart"`)

### "OPENAI_API_KEY not set"
Export your OpenAI API key:
```bash
export OPENAI_API_KEY="sk-..."
```

### Streaming hangs
The agent is thinking or executing a long command. Check sandbox logs:
```bash
docker logs sandkasten-<session_id>
```

---

## Next Steps

1. **Customize the agent** — Edit instructions and add domain knowledge
2. **Add more tools** — Create tools for APIs, databases, git operations
3. **Deploy** — Use the main daemon docker-compose for production
4. **Integrate** — Wire these patterns into your own application

---

## Learn More

- [OpenAI Agents SDK Docs](https://openai.github.io/openai-agents-python/)
- [Streaming Guide](https://openai.github.io/openai-agents-python/streaming/)
- [Sessions Guide](https://openai.github.io/openai-agents-python/sessions/)
- [Rich Documentation](https://rich.readthedocs.io/)
