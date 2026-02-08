/**
 * Resolve AI SDK model from playground settings (OpenAI, Google, Google Vertex).
 * Used with aisdk() from @openai/agents-extensions to drive the agent.
 */

import type { PlaygroundSettings } from './types';
import { DEFAULT_MODELS } from './types';

function getApiKey(settings: PlaygroundSettings): string {
	return settings.provider === 'openai'
		? settings.openaiApiKey
		: settings.provider === 'google'
			? settings.googleApiKey
			: settings.googleVertexApiKey;
}

/**
 * Returns the AI SDK model instance for the given settings.
 * Throws if the selected provider needs an API key and it's missing.
 * Return value is passed to aisdk() from @openai/agents-extensions.
 */
export async function getModel(settings: PlaygroundSettings): Promise<unknown> {
	const provider = settings.provider;
	const modelName = (settings.model?.trim() || DEFAULT_MODELS[provider]).trim();
	const apiKey = getApiKey(settings)?.trim();

	switch (provider) {
		case 'openai': {
			if (!apiKey) throw new Error('OpenAI API key is required. Set it in Settings.');
			const { createOpenAI } = await import('@ai-sdk/openai');
			const openai = createOpenAI({ apiKey });
			return openai(modelName);
		}
		case 'google': {
			if (!apiKey) {
				throw new Error('Google API key is required. Set it in Settings.');
			}
			const { createGoogleGenerativeAI } = await import('@ai-sdk/google');
			const google = createGoogleGenerativeAI({ apiKey });
			return google(modelName);
		}
		case 'google-vertex': {
			const { createVertex } = await import('@ai-sdk/google-vertex/edge');
			const project = settings.vertexProject?.trim();
			const location = settings.vertexLocation?.trim() || 'us-central1';

			if (apiKey && !project) {
				const vertex = createVertex({ apiKey });
				return vertex(modelName);
			}

			if (!project) {
				throw new Error(
					'Google Vertex project is required when not using Express mode. Set it in Settings or use Vertex API key only for Express mode.'
				);
			}

			const options: {
				project: string;
				location: string;
				googleCredentials?: { clientEmail: string; privateKey: string; privateKeyId?: string };
			} = { project, location };

			if (!apiKey) {
				throw new Error(
					'Google Vertex needs credentials: set Vertex API key (Express) or paste service account JSON in the API key field.'
				);
			}

			try {
				const creds = JSON.parse(apiKey) as {
					client_email?: string;
					private_key?: string;
					private_key_id?: string;
				};
				if (creds.client_email && creds.private_key) {
					options.googleCredentials = {
						clientEmail: creds.client_email,
						privateKey: creds.private_key,
						privateKeyId: creds.private_key_id
					};
				}
			} catch {
				const vertex = createVertex({ apiKey });
				return vertex(modelName);
			}

			const vertex = createVertex(options);
			return vertex(modelName);
		}
		default:
			throw new Error(`Unknown provider: ${provider}`);
	}
}

export function getApiKeyForProvider(settings: PlaygroundSettings): string {
	return getApiKey(settings);
}

/** Check if current settings have enough config to run (key or project where needed). */
export function hasRequiredConfig(settings: PlaygroundSettings): boolean {
	const key = getApiKey(settings)?.trim();
	if (settings.provider === 'openai') return !!key; // browser has no env
	if (settings.provider === 'google') return !!key;
	if (settings.provider === 'google-vertex') return !!key || !!settings.vertexProject?.trim();
	return false;
}
