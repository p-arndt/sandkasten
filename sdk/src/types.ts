export interface SandboxClientOptions {
  baseUrl: string;
  apiKey?: string;
}

export interface CreateSessionOptions {
  image?: string;
  ttlSeconds?: number;
}

export interface SessionInfo {
  id: string;
  image: string;
  status: string;
  cwd: string;
  created_at: string;
  expires_at: string;
}

export interface ExecResult {
  exit_code: number;
  cwd: string;
  output: string;
  truncated: boolean;
  duration_ms: number;
}

export interface ReadResult {
  path: string;
  content_base64: string;
  truncated: boolean;
}
