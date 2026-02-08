// Playground provider/model/keys persistence (localStorage, client-only)

import type { PlaygroundSettings } from './types';
import { DEFAULT_PLAYGROUND_SETTINGS } from './types';

const STORAGE_KEY = 'sandkasten_playground_settings';

export function getStoredPlaygroundSettings(): PlaygroundSettings {
	if (typeof window === 'undefined') return DEFAULT_PLAYGROUND_SETTINGS;
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return DEFAULT_PLAYGROUND_SETTINGS;
		const parsed = JSON.parse(raw) as Partial<PlaygroundSettings>;
		return { ...DEFAULT_PLAYGROUND_SETTINGS, ...parsed };
	} catch {
		return DEFAULT_PLAYGROUND_SETTINGS;
	}
}

export function setStoredPlaygroundSettings(settings: PlaygroundSettings): void {
	if (typeof window === 'undefined') return;
	localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
}
