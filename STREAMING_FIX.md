# Streaming Implementation Fix

## Summary

Fixed critical bug in streaming exec implementation where SSE `done` events were sent with incorrect (zero-valued) data.

## Bug Description

### Before Fix

The handler checked channel closure (`!ok`) to send the `done` event, but when a channel is closed, the received value is zero-valued. This caused:

```go
if !ok {
    // chunk is ZERO-VALUED here!
    fmt.Fprintf(w, "event: done\ndata: {\"exit_code\":%d,\"cwd\":\"%s\",\"duration_ms\":%d}\n\n",
        chunk.ExitCode,   // = 0 (wrong!)
        chunk.Cwd,        // = "" (wrong!)
        chunk.DurationMs) // = 0 (wrong!)
}
```

Result: Done events always had `exit_code=0`, `cwd=""`, `duration_ms=0` regardless of actual command results.

### After Fix

The handler now properly checks the `Done` flag and sends both chunk and done events as needed:

```go
if chunk.Done {
    // Send output as chunk event (if present)
    if chunk.Output != "" {
        fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", chunkJSON)
    }

    // Send completion event with actual metadata
    fmt.Fprintf(w, "event: done\ndata: {\"exit_code\":%d,\"cwd\":\"%s\",\"duration_ms\":%d}\n\n",
        chunk.ExitCode, chunk.Cwd, chunk.DurationMs) // ✓ Correct values!
}
```

## Behavior

### v0.1 (Current - Single Chunk)

**Manager sends:**
- 1 chunk: `{Done: true, Output: "full output", ExitCode: 0, Cwd: "/workspace", DurationMs: 1234}`

**Handler emits:**
- 1 `chunk` event with output
- 1 `done` event with metadata

**SDK yields:**
- `ExecChunk(output="full output", done=False)`
- `ExecChunk(output="", done=True, exit_code=0, cwd="/workspace", duration_ms=1234)`

### v0.2 (Future - True Streaming)

**Manager will send:**
- N chunks: `{Done: false, Output: "line 1\n", ...}`
- 1 final chunk: `{Done: true, Output: "", ExitCode: 0, ...}`

**Handler will emit:**
- N `chunk` events with incremental output
- 1 `done` event with metadata

**SDK will yield:**
- N `ExecChunk(output="line N", done=False)`
- 1 `ExecChunk(output="", done=True, exit_code=X, ...)`

## Alignment with Documentation

✅ **API Endpoint**: `POST /v1/sessions/{id}/exec/stream` - Implemented correctly
✅ **SSE Format**: Events (`chunk`, `done`, `error`) - Correct format
✅ **Python SDK**: `exec_stream()` method - Correctly parses events
✅ **Types**: `ExecChunk` dataclass - Matches spec
✅ **Limitations**: v0.1 note about single chunk - Accurate and implemented
✅ **Future-proof**: Handler supports both v0.1 and future v0.2 streaming

## Files Modified

- `internal/api/handlers.go` - Fixed `handleExecStream()` logic (lines 128-165)

## Verification

The implementation now correctly:
1. Sends command output in chunk events
2. Sends completion metadata in done events
3. Supports both single-chunk (v0.1) and multi-chunk (v0.2) modes
4. Aligns 100% with documentation at `docs/features/streaming.md`
