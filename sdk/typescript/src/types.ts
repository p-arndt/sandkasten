export interface SandboxClientOptions {
  baseUrl: string;
  apiKey?: string;
}

export interface CreateSessionOptions {
  image?: string;
  ttlSeconds?: number;
  workspaceId?: string;
}

export interface SessionInfo {
  id: string;
  image: string;
  status: string;
  cwd: string;
  created_at: string;
  expires_at: string;
  workspace_id?: string;
  last_activity?: string;
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

export interface WorkspaceInfo {
  id: string;
  created_at: string;
  session_count: number;
  active_session_count: number;
}

export interface ExecChunk {
  output: string;
  timestamp: number;
  exit_code?: number;
  cwd?: string;
  duration_ms?: number;
  done: boolean;
}
