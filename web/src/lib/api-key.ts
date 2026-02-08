// API key persistence and auth state for dashboard (localStorage, client-only)

import { writable } from 'svelte/store';

const STORAGE_KEY = 'sandkasten_api_key';

export interface AuthState {
	/** Whether an API key is stored (may still be invalid) */
	hasKey: boolean;
	/** Last request returned 401 — key missing or wrong */
	unauthorized: boolean;
}

export const authStore = writable<AuthState>({ hasKey: false, unauthorized: false });

export function getStoredApiKey(): string | null {
	if (typeof window === 'undefined') return null;
	return localStorage.getItem(STORAGE_KEY);
}

export function setStoredApiKey(key: string | null): void {
	if (typeof window === 'undefined') return;
	if (key === null || key === '') {
		localStorage.removeItem(STORAGE_KEY);
	} else {
		localStorage.setItem(STORAGE_KEY, key);
	}
	authStore.update((s) => ({ ...s, hasKey: !!key, unauthorized: false }));
}

export function setUnauthorized(value: boolean): void {
	authStore.update((s) => ({ ...s, unauthorized: value }));
}

/**
 * Load stored API key and apply it to the API client.
 * Call this once on app init (e.g. in root layout onMount).
 */
export function initApiKeyFromStorage(api: { setAPIKey: (key: string | null) => void }): void {
	const key = getStoredApiKey();
	api.setAPIKey(key);
	authStore.update((s) => ({ ...s, hasKey: !!key }));
}
