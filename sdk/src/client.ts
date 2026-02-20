import { Session } from "./session.js";
import type { SandboxClientOptions, CreateSessionOptions, SessionInfo, WorkspaceInfo } from "./types.js";

export class SandboxClient {
  private baseUrl: string;
  private apiKey?: string;

  constructor(opts: SandboxClientOptions) {
    this.baseUrl = opts.baseUrl.replace(/\/$/, "");
    this.apiKey = opts.apiKey;
  }

  async createSession(opts?: CreateSessionOptions): Promise<Session> {
    const res = await this.fetch("/v1/sessions", {
      method: "POST",
      body: JSON.stringify({
        image: opts?.image,
        ttl_seconds: opts?.ttlSeconds,
        workspace_id: opts?.workspaceId,
      }),
    });

    const info: SessionInfo = await res.json();
    return new Session(info.id, this);
  }

  async getSession(id: string): Promise<SessionInfo> {
    const res = await this.fetch(`/v1/sessions/${id}`);
    return res.json();
  }

  async listSessions(): Promise<SessionInfo[]> {
    const res = await this.fetch("/v1/sessions");
    return res.json();
  }

  async listWorkspaces(): Promise<WorkspaceInfo[]> {
    const res = await this.fetch("/v1/workspaces");
    const data = await res.json();
    return data.workspaces || [];
  }

  async deleteWorkspace(id: string): Promise<void> {
    await this.fetch(`/v1/workspaces/${id}`, {
      method: "DELETE",
    });
  }

  /** @internal */
  async fetch(path: string, init?: RequestInit): Promise<Response> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...(init?.headers as Record<string, string>),
    };

    if (this.apiKey) {
      headers["Authorization"] = `Bearer ${this.apiKey}`;
    }

    const res = await globalThis.fetch(`${this.baseUrl}${path}`, {
      ...init,
      headers,
    });

    if (!res.ok) {
      const body = await res.text();
      throw new Error(`sandkasten: ${res.status} ${res.statusText} — ${body}`);
    }

    return res;
  }
}
