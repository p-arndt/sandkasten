# Streaming Exec Output

Real-time command output streaming for long-running commands.

## Overview

By default, `exec()` is **blocking** - it returns when the command completes with all output at once. For long-running commands (pip install, large downloads, etc.), streaming provides real-time feedback.

## Modes

### Blocking (Default)

```python
result = await session.exec("pip install pandas")
print(result.output)  # All output at once
```

**When to use:**
- Quick commands (< 5s)
- When you need the full output together
- Simpler error handling

### Streaming (Real-time)

```python
async for chunk in session.exec_stream("pip install pandas"):
    print(chunk.output, end='', flush=True)
    if chunk.done:
        print(f"\nExit code: {chunk.exit_code}")
```

**When to use:**
- Long-running commands (pip install, npm install, downloads)
- Progress indication needed
- Interactive user feedback
- Large output that should be displayed incrementally

## API

### HTTP Endpoint

**Streaming:**
```http
POST /v1/sessions/{id}/exec/stream
Content-Type: application/json

{
  "cmd": "pip install pandas",
  "timeout_ms": 120000
}
```

**Response:** Server-Sent Events (SSE)

```
event: chunk
data: {"chunk":"Collecting pandas\n","timestamp":1707390000000}

event: chunk
data: {"chunk":"Downloading pandas-2.0.0...\n","timestamp":1707390001000}

event: done
data: {"exit_code":0,"cwd":"/workspace","duration_ms":5234}
```

**Events:**
- `chunk` - Output chunk with text and timestamp
- `done` - Command completed (includes exit code, cwd, duration)
- `error` - Error occurred

## SDK Usage

### Python

```python
from sandkasten import SandboxClient

client = SandboxClient(base_url="...", api_key="...")
session = await client.create_session()

# Blocking
result = await session.exec("echo hello")
print(result.output)

# Streaming
async for chunk in session.exec_stream("pip install requests"):
    print(chunk.output, end='', flush=True)

    if chunk.done:
        print(f"\nCompleted in {chunk.duration_ms}ms")
        print(f"Exit code: {chunk.exit_code}")
```

### TypeScript (TODO)

```typescript
// Blocking
const result = await session.exec('echo hello');
console.log(result.output);

// Streaming
for await (const chunk of session.execStream('pip install requests')) {
    process.stdout.write(chunk.output);

    if (chunk.done) {
        console.log(`\nExit code: ${chunk.exitCode}`);
    }
}
```

## Examples

### Progress Indicator

```python
async for chunk in session.exec_stream("pip install pandas numpy scipy"):
    # Show output with progress
    print(chunk.output, end='', flush=True)

    if chunk.done:
        if chunk.exit_code == 0:
            print("\n✅ Installation successful")
        else:
            print(f"\n❌ Installation failed (code {chunk.exit_code})")
```

### Collect & Display

```python
output_buffer = []

async for chunk in session.exec_stream("python long_running.py"):
    output_buffer.append(chunk.output)
    print(chunk.output, end='', flush=True)

    if chunk.done:
        full_output = "".join(output_buffer)
        # Process complete output
```

### Timeout Handling

```python
try:
    async for chunk in session.exec_stream("sleep 100", timeout_ms=5000):
        print(chunk.output, end='')
        if chunk.done:
            print(f"Exit: {chunk.exit_code}")
except Exception as e:
    print(f"Error: {e}")
```

### Interactive Progress

```python
from rich.progress import Progress, SpinnerColumn

with Progress(SpinnerColumn(), *Progress.get_default_columns()) as progress:
    task = progress.add_task("Installing...", total=None)

    async for chunk in session.exec_stream("pip install large-package"):
        if chunk.output:
            progress.update(task, description=chunk.output[:50])

        if chunk.done:
            progress.update(task, completed=True)
```

## Performance

| Feature | Blocking | Streaming |
|---------|----------|-----------|
| First byte latency | High (waits for completion) | Low (~50ms) |
| Memory usage | Buffers all output | Streams chunks |
| User feedback | None until done | Real-time |
| Simplicity | Simple | Slightly more complex |

## Limitations

### Current Implementation (v0.1)

**Note:** The current implementation sends output as a single chunk when complete. True line-by-line streaming from the runner is planned for v0.2.

This means:
- Streaming works but delivers output in one chunk
- Still useful for timeout handling and async patterns
- API is forward-compatible with true streaming

### Future (v0.2)

- True line-by-line streaming from runner
- PTY output chunking
- Configurable chunk size
- Backpressure handling

## Error Handling

Streaming errors are sent as SSE error events:

```python
try:
    async for chunk in session.exec_stream("invalid-command"):
        print(chunk.output)
        if chunk.done:
            break
except Exception as e:
    print(f"Stream error: {e}")
```

## Use Cases

### Package Installation

```python
# Show progress
async for chunk in session.exec_stream("pip install tensorflow"):
    print(chunk.output, end='', flush=True)
```

### Large Downloads

```python
async for chunk in session.exec_stream("wget https://large-file.zip"):
    # Show download progress
    print(chunk.output, end='', flush=True)
```

### Build Output

```python
async for chunk in session.exec_stream("npm run build"):
    # Stream build output to user
    print(chunk.output, end='', flush=True)
```

### Log Tailing

```python
async for chunk in session.exec_stream("tail -f /var/log/app.log", timeout_ms=60000):
    print(chunk.output, end='', flush=True)
    if some_condition:
        break  # Stop streaming
```

## Best Practices

### 1. Choose the Right Mode

```python
# Quick command? Use blocking
result = await session.exec("pwd")

# Long command? Use streaming
async for chunk in session.exec_stream("pip install pandas"):
    print(chunk.output, end='')
```

### 2. Handle Done Chunks

```python
async for chunk in session.exec_stream("command"):
    if chunk.done:
        # Check exit code, cwd, duration
        if chunk.exit_code != 0:
            raise Exception(f"Command failed: {chunk.exit_code}")
```

### 3. Set Appropriate Timeouts

```python
# Long-running command
async for chunk in session.exec_stream(
    "pip install large-package",
    timeout_ms=300000  # 5 minutes
):
    ...
```

### 4. Flush Output

```python
# Ensure real-time display
async for chunk in session.exec_stream("command"):
    print(chunk.output, end='', flush=True)  # ← flush=True
```

## Troubleshooting

### "Streaming not supported"

Your HTTP server or proxy doesn't support SSE. Check:
- Nginx: Add `proxy_buffering off;`
- Cloudflare: Disable buffering
- Use direct connection for testing

### Output arrives all at once

Expected in v0.1 (see Limitations). Still provides async benefits.

### Timeout too short

Increase timeout:
```python
async for chunk in session.exec_stream("cmd", timeout_ms=120000):
    ...
```

### Memory usage high

For very large output, process chunks instead of buffering:
```python
async for chunk in session.exec_stream("command"):
    process_chunk(chunk.output)  # Don't append to buffer
```
