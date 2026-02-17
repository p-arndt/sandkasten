// Playground provider/model/keys persistence (localStorage, client-only)

import { writable } from 'svelte/store';
import type { PlaygroundSettings } from './types';
import { DEFAULT_PLAYGROUND_SETTINGS } from './types';

const STORAGE_KEY = 'sandkasten_playground_settings';

/** In-memory provider keys when "Don't persist API keys" is enabled. Cleared on page reload. */
export const sessionOnlyKeys = writable<{
	openaiApiKey?: string;
	googleApiKey?: string;
	googleVertexApiKey?: string;
}>({});

/** Get settings from localStorage (client-only). Merges with sessionOnlyKeys for effective keys. */
export function getStoredPlaygroundSettings(): PlaygroundSettings {
	if (typeof window === 'undefined') return DEFAULT_PLAYGROUND_SETTINGS;
	try {
		const raw = sessionStorage.getItem(STORAGE_KEY);
		const base = raw
			? { ...DEFAULT_PLAYGROUND_SETTINGS, ...(JSON.parse(raw) as Partial<PlaygroundSettings>) }
			: { ...DEFAULT_PLAYGROUND_SETTINGS };
		return base as PlaygroundSettings;
	} catch {
		return DEFAULT_PLAYGROUND_SETTINGS;
	}
}

/** Get effective settings: stored + in-memory keys (e.g. from backend). */
export function getEffectivePlaygroundSettings(
	stored: PlaygroundSettings,
	sessionKeys: { openaiApiKey?: string; googleApiKey?: string; googleVertexApiKey?: string }
): PlaygroundSettings {
	return {
		...stored,
		openaiApiKey: sessionKeys.openaiApiKey ?? stored.openaiApiKey,
		googleApiKey: sessionKeys.googleApiKey ?? stored.googleApiKey,
		googleVertexApiKey: sessionKeys.googleVertexApiKey ?? stored.googleVertexApiKey
	};
}

/** Save settings. If persistProviderKeys is false, keys are not written (and sessionOnlyKeys should be set separately). */
export function setStoredPlaygroundSettings(settings: PlaygroundSettings): void {
	if (typeof window === 'undefined') return;
	const toSave = { ...settings };
	if (!toSave.persistProviderKeys) {
		toSave.openaiApiKey = '';
		toSave.googleApiKey = '';
		toSave.googleVertexApiKey = '';
	}
	sessionStorage.setItem(STORAGE_KEY, JSON.stringify(toSave));
}
