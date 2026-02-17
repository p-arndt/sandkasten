<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import './layout.css';
	import { page } from '$app/stores';
	import { ModeWatcher } from 'mode-watcher';
	import { Home, Terminal, Folder, Settings as SettingsIcon, Key, CheckCircle, AlertCircle, MessageSquare } from '@lucide/svelte';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import ModeToggle from '$lib/components/mode-toggle.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Toaster, toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { authStore, initApiKeyFromStorage, setStoredApiKey, setUnauthorized } from '$lib/api-key';

	let { children } = $props();
	let bannerApiKey = $state('');

	// Apply Sandkasten API key as soon as we're in the browser so child pages have it before their onMount
	if (browser) {
		initApiKeyFromStorage(api);
	}
	onMount(() => {
		if (!browser) initApiKeyFromStorage(api);
		api.setOnUnauthorized(() => setUnauthorized(true));
	});

	function saveBannerKey() {
		const key = bannerApiKey.trim() || null;
		setStoredApiKey(key);
		api.setAPIKey(key);
		bannerApiKey = '';
		toast.success(key ? 'Connected' : 'API key cleared');
	}

	const navItems = [
		{ href: '/', icon: Home, label: 'Overview' },
		{ href: '/playground', icon: MessageSquare, label: 'Playground' },
		{ href: '/sessions', icon: Terminal, label: 'Sessions' },
		{ href: '/workspaces', icon: Folder, label: 'Workspaces' },
		{ href: '/settings', icon: SettingsIcon, label: 'Settings' }
	];
</script>

<ModeWatcher />
<Toaster position="top-right" />

<Sidebar.Provider>
	<Sidebar.Sidebar>
		<Sidebar.Header>
			<div class="flex items-center gap-2 px-2 py-2">
				<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
					<span class="text-lg">🏖️</span>
				</div>
				<div class="flex flex-col">
					<span class="text-sm font-semibold">Sandkasten</span>
					<span class="text-xs text-muted-foreground">Runtime Dashboard</span>
				</div>
			</div>
		</Sidebar.Header>

		<Sidebar.Content>
			<Sidebar.Group>
				<Sidebar.GroupContent>
					<Sidebar.Menu>
						{#each navItems as item}
							<Sidebar.MenuItem>
								<Sidebar.MenuButton href={item.href} isActive={$page.url.pathname === item.href}>
									<item.icon class="h-4 w-4" />
									<span>{item.label}</span>
								</Sidebar.MenuButton>
							</Sidebar.MenuItem>
						{/each}
					</Sidebar.Menu>
				</Sidebar.GroupContent>
			</Sidebar.Group>
		</Sidebar.Content>

		<Sidebar.Footer>
			<div class="flex flex-col gap-1 px-2 py-2">
				<div class="flex items-center justify-between">
					<span class="text-xs text-muted-foreground">v0.0.1</span>
					<ModeToggle />
				</div>
				{#if $authStore.hasKey && !$authStore.unauthorized}
					<div class="flex items-center gap-1.5 text-xs text-green-600 dark:text-green-500">
						<CheckCircle class="h-3.5 w-3.5 shrink-0" />
						<span>Connected</span>
					</div>
				{:else}
					<a
						href="/settings"
						class="flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-500 hover:underline"
					>
						<AlertCircle class="h-3.5 w-3.5 shrink-0" />
						<span>{$authStore.unauthorized ? 'Invalid key' : 'Set API key'}</span>
					</a>
				{/if}
			</div>
		</Sidebar.Footer>
	</Sidebar.Sidebar>

	<Sidebar.Inset>
		<header class="sticky top-0 z-10 flex h-14 items-center gap-4 border-b bg-background px-6">
			<Sidebar.Trigger />
			<div class="flex-1"></div>
		</header>

		{#if !$authStore.hasKey || $authStore.unauthorized}
			<div
				class="flex flex-wrap items-center gap-3 border-b bg-muted/50 px-6 py-3 text-sm"
				role="region"
				aria-label="Connect to daemon"
			>
				<div class="flex items-center gap-2 font-medium">
					<Key class="h-4 w-4 text-muted-foreground" />
					{#if $authStore.unauthorized}
						<span>Invalid or missing API key — enter the key from your daemon config</span>
					{:else}
						<span>Connect to daemon: enter your API key</span>
					{/if}
				</div>
				<div class="flex items-center gap-2">
					<Input
						bind:value={bannerApiKey}
						type="password"
						placeholder="API key"
						class="h-8 w-48 font-mono text-xs"
						autocomplete="off"
					/>
					<Button size="sm" onclick={saveBannerKey}>
						Save & connect
					</Button>
				</div>
				<a href="/settings" class="text-muted-foreground underline hover:text-foreground">Settings</a>
			</div>
		{/if}

		<main class="flex-1 p-6">
			{@render children()}
		</main>
	</Sidebar.Inset>
</Sidebar.Provider>
