<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Terminal, Plus, Search, RefreshCw, Trash2, Copy, ExternalLink } from '@lucide/svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Select from '$lib/components/ui/select';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { api } from '$lib/api';
	import { formatRelativeTime, formatTimestamp } from '$lib/utils/time';
	import { copyToClipboard } from '$lib/utils/clipboard';
	import type { SessionInfo } from '$lib/types';
	import { toast } from 'svelte-sonner';

	let sessions = $state<SessionInfo[]>([]);
	let loading = $state(true);
	let searchQuery = $state('');
	let statusFilter = $state('all');
	let refreshInterval: number;

	// Create dialog state
	let createDialogOpen = $state(false);
	let createImage = $state('sandbox-runtime:python');
	let createTTL = $state(3600);
	let createWorkspace = $state('');
	let creating = $state(false);

	// Destroy dialog state
	let destroyDialogOpen = $state(false);
	let sessionToDestroy = $state<SessionInfo | null>(null);
	let destroying = $state(false);

	async function loadSessions() {
		try {
			sessions = await api.getSessions();
			loading = false;
		} catch (err) {
			toast.error('Failed to load sessions', {
				description: err instanceof Error ? err.message : 'Unknown error'
			});
			loading = false;
		}
	}

	async function createSession() {
		creating = true;
		try {
			const opts: any = {
				image: createImage,
				ttl_seconds: createTTL
			};
			if (createWorkspace) {
				opts.workspace_id = createWorkspace;
			}
			
			const session = await api.createSession(opts);
			toast.success('Session created', {
				description: `Session ${session.id.substring(0, 12)}... is ready`
			});
			createDialogOpen = false;
			loadSessions();
		} catch (err) {
			toast.error('Failed to create session', {
				description: err instanceof Error ? err.message : 'Unknown error'
			});
		} finally {
			creating = false;
		}
	}

	async function destroySession() {
		if (!sessionToDestroy) return;
		
		destroying = true;
		try {
			await api.destroySession(sessionToDestroy.id);
			toast.success('Session destroyed', {
				description: `Session ${sessionToDestroy.id.substring(0, 12)}... has been removed`
			});
			destroyDialogOpen = false;
			sessionToDestroy = null;
			loadSessions();
		} catch (err) {
			toast.error('Failed to destroy session', {
				description: err instanceof Error ? err.message : 'Unknown error'
			});
		} finally {
			destroying = false;
		}
	}

	function openDestroyDialog(session: SessionInfo) {
		sessionToDestroy = session;
		destroyDialogOpen = true;
	}

	async function copyId(id: string) {
		const success = await copyToClipboard(id);
		if (success) {
			toast.success('Copied to clipboard');
		} else {
			toast.error('Failed to copy');
		}
	}

	function getStatusColor(sessionStatus: string): string {
		switch (sessionStatus) {
			case 'running':
				return 'default';
			case 'expired':
				return 'secondary';
			case 'destroyed':
				return 'destructive';
			default:
				return 'outline';
		}
	}

	// Computed filtered sessions
	let filteredSessions = $derived(
		sessions.filter((session) => {
			const matchesSearch =
				session.id.toLowerCase().includes(searchQuery.toLowerCase()) ||
				session.image.toLowerCase().includes(searchQuery.toLowerCase());
			const matchesStatus = statusFilter === 'all' || session.status === statusFilter;
			return matchesSearch && matchesStatus;
		})
	);

	onMount(() => {
		loadSessions();
		refreshInterval = setInterval(loadSessions, 5000) as unknown as number;
	});

	onDestroy(() => {
		if (refreshInterval) clearInterval(refreshInterval);
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-3xl font-bold tracking-tight">Sessions</h1>
			<p class="text-muted-foreground">Manage your sandbox sessions</p>
		</div>
		<div class="flex gap-2">
			<Button onclick={loadSessions} variant="outline" size="sm">
				<RefreshCw class="mr-2 h-4 w-4" />
				Refresh
			</Button>
			<Button onclick={() => (createDialogOpen = true)} size="sm">
				<Plus class="mr-2 h-4 w-4" />
				Create Session
			</Button>
		</div>
	</div>

	<!-- Filters -->
	<Card.Root>
		<Card.Content class="pt-6">
			<div class="flex gap-4">
				<div class="relative flex-1">
					<Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
					<Input
						type="text"
						placeholder="Search sessions by ID or image..."
						class="pl-9"
						bind:value={searchQuery}
					/>
				</div>
				<Select.Root type="single" bind:value={statusFilter}>
					<Select.Trigger class="w-[180px]">
						{statusFilter === 'all' ? 'All Status' : statusFilter.charAt(0).toUpperCase() + statusFilter.slice(1)}
					</Select.Trigger>
					<Select.Content>
						<Select.Item value="all" label="All Status">All Status</Select.Item>
						<Select.Item value="running" label="Running">Running</Select.Item>
						<Select.Item value="expired" label="Expired">Expired</Select.Item>
						<Select.Item value="destroyed" label="Destroyed">Destroyed</Select.Item>
					</Select.Content>
				</Select.Root>
			</div>
		</Card.Content>
	</Card.Root>

	<!-- Sessions Table -->
	<Card.Root>
		<Card.Content class="pt-6">
			{#if loading}
				<div class="space-y-2">
					{#each Array(5) as _}
						<Skeleton class="h-12 w-full" />
					{/each}
				</div>
			{:else if filteredSessions.length > 0}
				<div class="rounded-md border">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>ID</Table.Head>
								<Table.Head>Image</Table.Head>
								<Table.Head>Status</Table.Head>
								<Table.Head>Workspace</Table.Head>
								<Table.Head>Created</Table.Head>
								<Table.Head>Expires</Table.Head>
								<Table.Head>Last Activity</Table.Head>
								<Table.Head class="text-right">Actions</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each filteredSessions as session}
								<Table.Row>
									<Table.Cell class="font-mono text-xs">
										<button
											onclick={() => copyId(session.id)}
											class="group flex items-center gap-2 hover:text-primary"
											title="Click to copy full ID"
										>
											{session.id.substring(0, 12)}...
											<Copy class="h-3 w-3 opacity-0 transition-opacity group-hover:opacity-100" />
										</button>
									</Table.Cell>
									<Table.Cell>{session.image}</Table.Cell>
									<Table.Cell>
										<Badge variant={getStatusColor(session.status)}>{session.status}</Badge>
									</Table.Cell>
									<Table.Cell class="font-mono text-xs">
										{session.workspace_id ? session.workspace_id.substring(0, 12) + '...' : '-'}
									</Table.Cell>
									<Table.Cell class="text-muted-foreground" title={formatTimestamp(session.created_at)}>
										{formatRelativeTime(session.created_at)}
									</Table.Cell>
									<Table.Cell class="text-muted-foreground" title={formatTimestamp(session.expires_at)}>
										{formatRelativeTime(session.expires_at)}
									</Table.Cell>
									<Table.Cell class="text-muted-foreground" title={session.last_activity ? formatTimestamp(session.last_activity) : '-'}>
										{session.last_activity ? formatRelativeTime(session.last_activity) : '-'}
									</Table.Cell>
									<Table.Cell class="text-right">
										<Button
											variant="ghost"
											size="sm"
											onclick={() => openDestroyDialog(session)}
											disabled={session.status === 'destroyed'}
										>
											<Trash2 class="h-4 w-4 text-destructive" />
										</Button>
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
			{:else}
				<div class="flex flex-col items-center justify-center py-12 text-center">
					<Terminal class="mb-2 h-8 w-8 text-muted-foreground" />
					<p class="text-sm text-muted-foreground">
						{searchQuery || statusFilter !== 'all' ? 'No sessions match your filters' : 'No sessions yet'}
					</p>
				</div>
			{/if}
		</Card.Content>
	</Card.Root>
</div>

<!-- Create Session Dialog -->
<Dialog.Root bind:open={createDialogOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Create Session</Dialog.Title>
			<Dialog.Description>Create a new sandbox session with custom configuration</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-4 py-4">
			<div class="space-y-2">
				<Label for="image">Docker Image</Label>
				<Select.Root type="single" bind:value={createImage}>
					<Select.Trigger id="image" class="w-full">
						{createImage}
					</Select.Trigger>
					<Select.Content>
						<Select.Item value="sandbox-runtime:base" label="sandbox-runtime:base">
							sandbox-runtime:base
						</Select.Item>
						<Select.Item value="sandbox-runtime:python" label="sandbox-runtime:python">
							sandbox-runtime:python
						</Select.Item>
						<Select.Item value="sandbox-runtime:node" label="sandbox-runtime:node">
							sandbox-runtime:node
						</Select.Item>
					</Select.Content>
				</Select.Root>
			</div>
			<div class="space-y-2">
				<Label for="ttl">TTL (seconds)</Label>
				<Input id="ttl" type="number" bind:value={createTTL} min="60" max="86400" />
				<p class="text-xs text-muted-foreground">Session lifetime (60s - 24h)</p>
			</div>
			<div class="space-y-2">
				<Label for="workspace">Workspace ID (optional)</Label>
				<Input id="workspace" type="text" bind:value={createWorkspace} placeholder="Leave empty for ephemeral" />
				<p class="text-xs text-muted-foreground">Use existing workspace or create ephemeral</p>
			</div>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (createDialogOpen = false)} disabled={creating}>
				Cancel
			</Button>
			<Button onclick={createSession} disabled={creating}>
				{creating ? 'Creating...' : 'Create Session'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Destroy Confirmation Dialog -->
<AlertDialog.Root bind:open={destroyDialogOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Destroy Session?</AlertDialog.Title>
			<AlertDialog.Description>
				This will permanently destroy session
				<code class="rounded bg-muted px-1 py-0.5 text-xs">
					{sessionToDestroy?.id.substring(0, 12)}...
				</code>
				and remove its container. This action cannot be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={destroying}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action onclick={destroySession} disabled={destroying}>
				{destroying ? 'Destroying...' : 'Destroy'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
