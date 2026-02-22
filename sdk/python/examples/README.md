# Examples

## openai_agents/

Examples for Sandkasten + OpenAI Agents SDK. Requires `pip install sandkasten[agents]`.

### openai_agents_example.py

Minimal example: create session, get tools, run an agent.

```bash
python examples/openai_agents/openai_agents_example.py
```

### workspace_example.py

Multi-user workspace demo: each user gets an isolated workspace. Files persist across sessions for the same `workspace_id`. User A never sees user B's files.

```bash
python examples/openai_agents/workspace_example.py
```

Shows:
- User alice creates a file
- User bob lists files (Alice's file not visible)
- User alice's file persists in a new session

---

Environment variables (optional):
- `SANDKASTEN_BASE_URL` — default `http://localhost:8080`
- `SANDKASTEN_API_KEY` — default `sk-sandbox-quickstart`
