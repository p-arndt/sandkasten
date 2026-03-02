import { describe, it, expect, beforeEach, vi } from "vitest";
import { SandboxClient } from "../src/client.js";
import { Session } from "../src/session.js";
import type { SessionInfo, WorkspaceInfo } from "../src/types.js";

// simple helper to create a fake Response
function makeResponse(body: any, ok = true, status = 200) {
  return {
    ok,
    status,
    statusText: ok ? "OK" : "Error",
    json: async () => body,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

describe("SandboxClient", () => {
  beforeEach(() => {
    // reset global fetch mock
    globalThis.fetch = vi.fn();
  });

  it("createSession returns Session with ID and normalises baseUrl", async () => {
    const payload = { id: "abc12345-678", image: "python", status: "running" };
    (globalThis.fetch as unknown as vi.Mock).mockResolvedValue(makeResponse(payload));

    const client = new SandboxClient({ baseUrl: "https://api.example.com/", apiKey: "key" });
    const session = await client.createSession({ image: "python" });

    expect(session.id).toBe("abc12345-678");

    // ensure fetch called with cleaned URL and correct body
    expect(globalThis.fetch).toHaveBeenCalledWith(
      "https://api.example.com/v1/sessions",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ image: "python", ttl_seconds: undefined, workspace_id: undefined }),
      })
    );
  });

  it("listSessions handles raw array and wrapped format", async () => {
    const raw: SessionInfo[] = [
      {
        id: "s1",
        image: "python",
        status: "running",
        cwd: "/workspace",
        created_at: "2025-01-01T00:00:00Z",
        expires_at: "2025-01-02T00:00:00Z",
      },
    ];

    (globalThis.fetch as unknown as vi.Mock).mockResolvedValue(makeResponse(raw));
    const client = new SandboxClient({ baseUrl: "http://x", apiKey: "k" });
    let sessions = await client.listSessions();
    expect(sessions).toHaveLength(1);
    expect(sessions[0].id).toBe("s1");

    // wrapped format
    (globalThis.fetch as unknown as vi.Mock).mockResolvedValue(makeResponse({ sessions: raw }));
    sessions = await client.listSessions();
    expect(sessions[0].id).toBe("s1");
  });

  it("listWorkspaces returns WorkspaceInfo[] and handles empty result", async () => {
    const workspaces: WorkspaceInfo[] = [{ id: "ws-1", created_at: "", session_count: 0, active_session_count: 0 }];
    (globalThis.fetch as unknown as vi.Mock).mockResolvedValue(makeResponse({ workspaces }));

    const client = new SandboxClient({ baseUrl: "http://x", apiKey: "k" });
    const w = await client.listWorkspaces();
    expect(w).toEqual(workspaces);
  });

  describe("Session class", () => {
    let client: SandboxClient;
    let session: Session;

    beforeEach(() => {
      client = new SandboxClient({ baseUrl: "http://x", apiKey: "k" });
      session = new Session("id", client);
    });

    it("exec posts to correct path and returns ExecResult", async () => {
      const result = { exit_code: 0, cwd: "/", output: "ok", truncated: false, duration_ms: 10 };
      (globalThis.fetch as unknown as vi.Mock).mockResolvedValue(makeResponse(result));
      const res = await session.exec("ls", { timeoutMs: 500 });
      expect(res.exit_code).toBe(0);
      expect(globalThis.fetch).toHaveBeenCalledWith(
        "http://x/v1/sessions/id/exec",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ cmd: "ls", timeout_ms: 500, raw_output: undefined }),
        })
      );
    });

    it("read and write convert correctly and include query param", async () => {
      const readResult = { path: "/foo", content_base64: "Zm9v", truncated: false };
      (globalThis.fetch as unknown as vi.Mock).mockResolvedValue(makeResponse(readResult));
      const bytes = await session.read("/foo", { maxBytes: 10 });
      expect(bytes).toBeInstanceOf(Uint8Array);
      expect(globalThis.fetch).toHaveBeenCalledWith(
        "http://x/v1/sessions/id/fs/read?path=%2Ffoo&max_bytes=10",
        expect.anything()
      );

      // write as text
      (globalThis.fetch as unknown as vi.Mock).mockResolvedValue(makeResponse({ ok: true }));
      await session.write("/bar", "hello");
      expect(globalThis.fetch).toHaveBeenCalledWith(
        "http://x/v1/sessions/id/fs/write",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ path: "/bar", text: "hello" }),
        })
      );
    });

    it("info and destroy call correct endpoints", async () => {
      (globalThis.fetch as unknown as vi.Mock).mockResolvedValue(makeResponse({ id: "id" }));
      const info = await session.info();
      expect(info.id).toBe("id");
      expect(globalThis.fetch).toHaveBeenCalledWith("http://x/v1/sessions/id", expect.anything());

      (globalThis.fetch as unknown as vi.Mock).mockResolvedValue(makeResponse({ ok: true }));
      await session.destroy();
      expect(globalThis.fetch).toHaveBeenCalledWith("http://x/v1/sessions/id", expect.objectContaining({ method: "DELETE" }));
    });
  });
});
