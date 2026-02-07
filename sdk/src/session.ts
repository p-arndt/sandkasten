import type { SandboxClient } from "./client.js";
import type { ExecResult, ReadResult } from "./types.js";

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
    opts?: { timeoutMs?: number }
  ): Promise<ExecResult> {
    const res = await this.client.fetch(`/v1/sessions/${this.id}/exec`, {
      method: "POST",
      body: JSON.stringify({
        cmd,
        timeout_ms: opts?.timeoutMs,
      }),
    });
    return res.json();
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
