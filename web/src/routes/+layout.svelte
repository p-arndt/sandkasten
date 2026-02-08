<script lang="ts">
	import './layout.css';
	import { page } from '$app/stores';
	import { ModeWatcher } from 'mode-watcher';
	import { Home, Terminal, Folder, Settings as SettingsIcon } from '@lucide/svelte';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import ModeToggle from '$lib/components/mode-toggle.svelte';
	import { Toaster } from 'svelte-sonner';

	let { children } = $props();

	const navItems = [
		{ href: '/', icon: Home, label: 'Overview' },
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
			<div class="flex items-center justify-between px-2 py-2">
				<div class="text-xs text-muted-foreground">v0.0.1</div>
				<ModeToggle />
			</div>
		</Sidebar.Footer>
	</Sidebar.Sidebar>

	<Sidebar.Inset>
		<header class="sticky top-0 z-10 flex h-14 items-center gap-4 border-b bg-background px-6">
			<Sidebar.Trigger />
			<div class="flex-1"></div>
		</header>
		
		<main class="flex-1 p-6">
			{@render children()}
		</main>
	</Sidebar.Inset>
</Sidebar.Provider>
