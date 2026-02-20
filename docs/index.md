# Sandkasten Documentation

Welcome to the Sandkasten docs. This page is the entry point for all guides and references.

## Getting started

| Guide                                 | Description                                                             |
| ------------------------------------- | ----------------------------------------------------------------------- |
| [Quickstart](quickstart.md)           | Get running in 5 minutes — build, images, config, daemon, first session |
| [Windows / WSL2](windows.md)          | Run Sandkasten on Windows via WSL2                                      |
| [OpenAI Agents SDK](openai-agents.md) | Use Sandkasten as tools (exec, read, write) with the OpenAI Agents SDK  |

## Reference

| Guide                             | Description                                         |
| --------------------------------- | --------------------------------------------------- |
| [API Reference](api.md)           | Complete HTTP API documentation                     |
| [Configuration](configuration.md) | Config file options, env vars, and image management |
| [Security Guide](security.md)     | Hardened config, seccomp, and security validation   |

## Features

| Guide                                           | Description                                                |
| ----------------------------------------------- | ---------------------------------------------------------- |
| [Persistent Workspaces](features/workspaces.md) | Directory-backed storage that survives session destruction |
| [Streaming Exec](features/streaming.md)         | Real-time command output for long-running commands         |
| [Session Pool](features/pool.md)                | Pre-warmed session pool for lower latency                  |

---

Start with [Quickstart](quickstart.md) if you're new. For production, read [Configuration](configuration.md) and [Security Guide](security.md).
