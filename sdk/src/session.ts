import type { SandboxClient } from "./client.js";
import type { ExecResult, ReadResult, SessionInfo, ExecChunk } from "./types.js";

export class Session {
  readonly id: string;
  private client: SandboxClient;

  /** @internal */
  constructor(id: string, client: SandboxClient) {
    this.id = id;
    this.client = client;
  }

  async exec(
    cmd: string,
    opts?: { timeoutMs?: number; rawOutput?: boolean }
  ): Promise<ExecResult> {
    const res = await this.client.fetch(`/v1/sessions/${this.id}/exec`, {
      method: "POST",
      body: JSON.stringify({
        cmd,
        timeout_ms: opts?.timeoutMs,
        raw_output: opts?.rawOutput,
      }),
    });
    return res.json();
  }

  async *execStream(
    cmd: string,
    opts?: { timeoutMs?: number; rawOutput?: boolean }
  ): AsyncIterableIterator<ExecChunk> {
    const res = await this.client.fetch(`/v1/sessions/${this.id}/exec/stream`, {
      method: "POST",
      body: JSON.stringify({
        cmd,
        timeout_ms: opts?.timeoutMs,
        raw_output: opts?.rawOutput,
      }),
    });

    if (!res.body) {
      throw new Error("Response body is null");
    }

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    try {
      while (true) {
        const { done, value } = await reader.read();

        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || ""; // Keep incomplete line in buffer

        let eventType = "";
        for (const line of lines) {
          if (line.startsWith("event:")) {
            eventType = line.substring(6).trim();
          } else if (line.startsWith("data:")) {
            const data = line.substring(5).trim();
            if (!data) continue;

            const parsed = JSON.parse(data);

            if (eventType === "chunk") {
              yield {
                output: parsed.chunk || "",
                timestamp: parsed.timestamp || Date.now(),
                done: false,
              };
            } else if (eventType === "done") {
              yield {
                output: "",
                timestamp: Date.now(),
                exit_code: parsed.exit_code,
                cwd: parsed.cwd,
                duration_ms: parsed.duration_ms,
                done: true,
              };
              return;
            } else if (eventType === "error") {
              throw new Error(parsed.error || "Unknown streaming error");
            }

            eventType = ""; // Reset for next event
          }
        }
      }
    } finally {
      reader.releaseLock();
    }
  }

  async write(
    path: string,
    content: string | Uint8Array
  ): Promise<void> {
    const body: Record<string, string> = { path };

    if (typeof content === "string") {
      body.text = content;
    } else {
      body.content_base64 = uint8ArrayToBase64(content);
    }

    await this.client.fetch(`/v1/sessions/${this.id}/fs/write`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async read(
    path: string,
    opts?: { maxBytes?: number }
  ): Promise<Uint8Array> {
    const params = new URLSearchParams({ path });
    if (opts?.maxBytes) {
      params.set("max_bytes", String(opts.maxBytes));
    }

    const res = await this.client.fetch(
      `/v1/sessions/${this.id}/fs/read?${params}`
    );
    const data: ReadResult = await res.json();
    return base64ToUint8Array(data.content_base64);
  }

  async readText(
    path: string,
    opts?: { maxBytes?: number }
  ): Promise<string> {
    const bytes = await this.read(path, opts);
    return new TextDecoder().decode(bytes);
  }

  async writeText(path: string, text: string): Promise<void> {
    await this.write(path, text);
  }

  async info(): Promise<SessionInfo> {
    const res = await this.client.fetch(`/v1/sessions/${this.id}`);
    return res.json();
  }

  async destroy(): Promise<void> {
    await this.client.fetch(`/v1/sessions/${this.id}`, {
      method: "DELETE",
    });
  }
}

function uint8ArrayToBase64(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary);
}

function base64ToUint8Array(base64: string): Uint8Array {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}
