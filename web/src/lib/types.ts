// API Types for Sandkasten

export interface SessionInfo {
	id: string;
	image: string;
	status: 'running' | 'expired' | 'destroyed';
	cwd: string;
	workspace_id?: string;
	created_at: string;
	expires_at: string;
	last_activity?: string;
}

export interface ExecResult {
	exit_code: number;
	cwd: string;
	output: string;
	truncated: boolean;
	duration_ms: number;
}

export interface ExecChunk {
	output: string;
	timestamp: number;
	exit_code?: number;
	cwd?: string;
	duration_ms?: number;
	done?: boolean;
}

export interface Workspace {
	id: string;
	created_at: string;
	size_mb: number;
	labels?: Record<string, string>;
}

export interface StatusResponse {
	total_sessions: number;
	active_sessions: number;
	expired_sessions: number;
	uptime_seconds: number;
	sessions: SessionInfo[];
	pool?: PoolStatus;
}

export interface PoolStatus {
	enabled: boolean;
	images: Record<string, ImageStatus>;
}

export interface ImageStatus {
	target: number;
	current: number;
}

export interface ConfigResponse {
	content: string;
	path: string;
}

export interface CreateSessionOpts {
	image?: string;
	ttl_seconds?: number;
	workspace_id?: string;
}

export interface ExecOpts {
	cmd: string;
	timeout_ms?: number;
}

export interface WriteFileOpts {
	path: string;
	text?: string;
	content_base64?: string;
}

export interface ReadFileResponse {
	path: string;
	content_base64: string;
	truncated: boolean;
}

export interface APIError {
	error_code?: string;
	message: string;
	details?: Record<string, any>;
}
