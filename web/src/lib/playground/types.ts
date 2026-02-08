// Playground chat and stream event types

export type ProviderId = 'openai' | 'google' | 'google-vertex';

export interface PlaygroundSettings {
	provider: ProviderId;
	model: string;
	openaiApiKey: string;
	googleApiKey: string;
	googleVertexApiKey: string;
	vertexProject: string;
	vertexLocation: string;
}

export const DEFAULT_PLAYGROUND_SETTINGS: PlaygroundSettings = {
	provider: 'openai',
	model: 'gpt-4.1',
	openaiApiKey: '',
	googleApiKey: '',
	googleVertexApiKey: '',
	vertexProject: '',
	vertexLocation: 'us-central1'
};

export const PROVIDER_LABELS: Record<ProviderId, string> = {
	openai: 'OpenAI',
	google: 'Google (Gemini)',
	'google-vertex': 'Google Vertex AI'
};

export const DEFAULT_MODELS: Record<ProviderId, string> = {
	openai: 'gpt-4.1',
	google: 'gemini-2.0-flash',
	'google-vertex': 'gemini-2.0-flash'
};

export interface PlaygroundContext {
	sessionId: string;
}

export type ChatRole = 'user' | 'assistant';

export interface ToolCallDisplay {
	id: string;
	name: string;
	args: Record<string, unknown>;
	output?: string;
	error?: string;
	status: 'pending' | 'running' | 'done';
}

export interface ChatMessage {
	id: string;
	role: ChatRole;
	content: string;
	toolCalls?: ToolCallDisplay[];
	streaming?: boolean;
}

export type StreamEventKind =
	| { type: 'text_delta'; delta: string }
	| { type: 'tool_call'; id: string; name: string; args: Record<string, unknown> }
	| { type: 'tool_result'; id: string; output: string }
	| { type: 'tool_error'; id: string; error: string }
	| { type: 'done' }
	| { type: 'error'; message: string };
