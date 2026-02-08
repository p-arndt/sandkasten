// API Client for Sandkasten

import type {
	SessionInfo,
	ExecResult,
	Workspace,
	StatusResponse,
	ConfigResponse,
	CreateSessionOpts,
	ExecOpts,
	WriteFileOpts,
	ReadFileResponse,
	APIError
} from './types';

type UnauthorizedCallback = () => void;

class SandkastenAPI {
	private baseURL: string;
	private apiKey: string | null = null;
	private onUnauthorized: UnauthorizedCallback | null = null;

	constructor(baseURL = '') {
		this.baseURL = baseURL;
	}

	setAPIKey(key: string | null) {
		this.apiKey = key;
	}

	/** Called when a request returns 401; use to show "enter API key" UI */
	setOnUnauthorized(cb: UnauthorizedCallback | null) {
		this.onUnauthorized = cb;
	}

	private async request<T>(
		endpoint: string,
		options: RequestInit = {}
	): Promise<T> {
		const headers: HeadersInit = {
			'Content-Type': 'application/json',
			...options.headers
		};

		if (this.apiKey) {
			headers['Authorization'] = `Bearer ${this.apiKey}`;
		}

		const response = await fetch(`${this.baseURL}${endpoint}`, {
			...options,
			headers
		});

		if (response.status === 401) {
			this.onUnauthorized?.();
		}

		if (!response.ok) {
			let error: APIError;
			try {
				error = await response.json();
			} catch {
				error = { message: `HTTP ${response.status}: ${response.statusText}` };
			}
			throw new Error(error.message);
		}

		return response.json();
	}

	// Dashboard Status
	async getStatus(): Promise<StatusResponse> {
		return this.request<StatusResponse>('/api/status');
	}

	// Sessions
	async getSessions(): Promise<SessionInfo[]> {
		return this.request<SessionInfo[]>('/v1/sessions');
	}

	async getSession(id: string): Promise<SessionInfo> {
		return this.request<SessionInfo>(`/v1/sessions/${id}`);
	}

	async createSession(opts: CreateSessionOpts = {}): Promise<SessionInfo> {
		return this.request<SessionInfo>('/v1/sessions', {
			method: 'POST',
			body: JSON.stringify(opts)
		});
	}

	async destroySession(id: string): Promise<{ ok: boolean }> {
		return this.request<{ ok: boolean }>(`/v1/sessions/${id}`, {
			method: 'DELETE'
		});
	}

	// Execution
	async execCommand(id: string, opts: ExecOpts): Promise<ExecResult> {
		return this.request<ExecResult>(`/v1/sessions/${id}/exec`, {
			method: 'POST',
			body: JSON.stringify(opts)
		});
	}

	// File System
	async writeFile(
		sessionId: string,
		opts: WriteFileOpts
	): Promise<{ ok: boolean }> {
		return this.request<{ ok: boolean }>(
			`/v1/sessions/${sessionId}/fs/write`,
			{
				method: 'POST',
				body: JSON.stringify(opts)
			}
		);
	}

	async readFile(
		sessionId: string,
		path: string,
		maxBytes?: number
	): Promise<ReadFileResponse> {
		const params = new URLSearchParams({ path });
		if (maxBytes !== undefined) {
			params.set('max_bytes', maxBytes.toString());
		}
		return this.request<ReadFileResponse>(
			`/v1/sessions/${sessionId}/fs/read?${params}`
		);
	}

	// Workspaces
	async getWorkspaces(): Promise<{ workspaces: Workspace[] }> {
		return this.request<{ workspaces: Workspace[] }>('/v1/workspaces');
	}

	async deleteWorkspace(id: string): Promise<{ ok: boolean }> {
		return this.request<{ ok: boolean }>(`/v1/workspaces/${id}`, {
			method: 'DELETE'
		});
	}

	// Configuration
	async getConfig(): Promise<ConfigResponse> {
		return this.request<ConfigResponse>('/api/config');
	}

	async saveConfig(content: string): Promise<{ success: boolean }> {
		return this.request<{ success: boolean }>('/api/config', {
			method: 'PUT',
			body: JSON.stringify({ content })
		});
	}

	async validateConfig(content: string): Promise<{ valid: boolean; error?: string }> {
		return this.request<{ valid: boolean; error?: string }>(
			'/api/config/validate',
			{
				method: 'POST',
				body: JSON.stringify({ content })
			}
		);
	}

	// Health Check
	async healthCheck(): Promise<{ status: string }> {
		return this.request<{ status: string }>('/healthz');
	}
}

// Export singleton instance
export const api = new SandkastenAPI();
