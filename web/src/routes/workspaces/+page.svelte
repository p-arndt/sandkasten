<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Folder, RefreshCw, Trash2, Copy, HardDrive } from '@lucide/svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { api } from '$lib/api';
	import { formatRelativeTime, formatTimestamp } from '$lib/utils/time';
	import { copyToClipboard } from '$lib/utils/clipboard';
	import type { Workspace } from '$lib/types';
	import { toast } from 'svelte-sonner';

	let workspaces = $state<Workspace[]>([]);
	let loading = $state(true);
	let refreshInterval: number;

	// Delete dialog state
	let deleteDialogOpen = $state(false);
	let workspaceToDelete = $state<Workspace | null>(null);
	let deleting = $state(false);

	async function loadWorkspaces() {
		try {
			const response = await api.getWorkspaces();
			workspaces = response.workspaces || [];
			loading = false;
		} catch (err) {
			toast.error('Failed to load workspaces', {
				description: err instanceof Error ? err.message : 'Unknown error'
			});
			loading = false;
		}
	}

	async function deleteWorkspace() {
		if (!workspaceToDelete) return;
		
		deleting = true;
		try {
			await api.deleteWorkspace(workspaceToDelete.id);
			toast.success('Workspace deleted', {
				description: `Workspace ${workspaceToDelete.id.substring(0, 12)}... has been removed`
			});
			deleteDialogOpen = false;
			workspaceToDelete = null;
			loadWorkspaces();
		} catch (err) {
			toast.error('Failed to delete workspace', {
				description: err instanceof Error ? err.message : 'Unknown error'
			});
		} finally {
			deleting = false;
		}
	}

	function openDeleteDialog(workspace: Workspace) {
		workspaceToDelete = workspace;
		deleteDialogOpen = true;
	}

	async function copyId(id: string) {
		const success = await copyToClipboard(id);
		if (success) {
			toast.success('Copied to clipboard');
		} else {
			toast.error('Failed to copy');
		}
	}

	function formatSize(sizeMB: number): string {
		if (sizeMB < 1024) {
			return `${sizeMB.toFixed(2)} MB`;
		}
		return `${(sizeMB / 1024).toFixed(2)} GB`;
	}

	onMount(() => {
		loadWorkspaces();
		refreshInterval = setInterval(loadWorkspaces, 10000) as unknown as number;
	});

	onDestroy(() => {
		if (refreshInterval) clearInterval(refreshInterval);
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-3xl font-bold tracking-tight">Workspaces</h1>
			<p class="text-muted-foreground">Manage persistent workspace volumes</p>
		</div>
		<Button onclick={loadWorkspaces} variant="outline" size="sm">
			<RefreshCw class="mr-2 h-4 w-4" />
			Refresh
		</Button>
	</div>

	<!-- Workspaces Table -->
	<Card.Root>
		<Card.Content class="pt-6">
			{#if loading}
				<div class="space-y-2">
					{#each Array(3) as _}
						<Skeleton class="h-12 w-full" />
					{/each}
				</div>
			{:else if workspaces.length > 0}
				<div class="rounded-md border">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>ID</Table.Head>
								<Table.Head>Created</Table.Head>
								<Table.Head>Size</Table.Head>
								<Table.Head>Labels</Table.Head>
								<Table.Head class="text-right">Actions</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each workspaces as workspace}
								<Table.Row>
									<Table.Cell class="font-mono text-xs">
										<button
											onclick={() => copyId(workspace.id)}
											class="group flex items-center gap-2 hover:text-primary"
											title="Click to copy full ID"
										>
											{workspace.id.substring(0, 12)}...
											<Copy class="h-3 w-3 opacity-0 transition-opacity group-hover:opacity-100" />
										</button>
									</Table.Cell>
									<Table.Cell class="text-muted-foreground" title={formatTimestamp(workspace.created_at)}>
										{formatRelativeTime(workspace.created_at)}
									</Table.Cell>
									<Table.Cell>
										<div class="flex items-center gap-2">
											<HardDrive class="h-4 w-4 text-muted-foreground" />
											{formatSize(workspace.size_mb)}
										</div>
									</Table.Cell>
									<Table.Cell>
										{#if workspace.labels && Object.keys(workspace.labels).length > 0}
											<div class="flex flex-wrap gap-1">
												{#each Object.entries(workspace.labels).slice(0, 3) as [key, value]}
													<Badge variant="outline" class="text-xs">
														{key}: {value}
													</Badge>
												{/each}
												{#if Object.keys(workspace.labels).length > 3}
													<Badge variant="outline" class="text-xs">
														+{Object.keys(workspace.labels).length - 3}
													</Badge>
												{/if}
											</div>
										{:else}
											<span class="text-muted-foreground">-</span>
										{/if}
									</Table.Cell>
									<Table.Cell class="text-right">
										<Button
											variant="ghost"
											size="sm"
											onclick={() => openDeleteDialog(workspace)}
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
					<Folder class="mb-2 h-8 w-8 text-muted-foreground" />
					<p class="text-sm text-muted-foreground">No persistent workspaces yet</p>
					<p class="mt-1 text-xs text-muted-foreground">
						Create a session with a workspace_id to persist data
					</p>
				</div>
			{/if}
		</Card.Content>
	</Card.Root>
</div>

<!-- Delete Confirmation Dialog -->
<AlertDialog.Root bind:open={deleteDialogOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete Workspace?</AlertDialog.Title>
			<AlertDialog.Description>
				This will permanently delete workspace
				<code class="rounded bg-muted px-1 py-0.5 text-xs">
					{workspaceToDelete?.id.substring(0, 12)}...
				</code>
				and all its data ({workspaceToDelete ? formatSize(workspaceToDelete.size_mb) : ''}). This action cannot be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action onclick={deleteWorkspace} disabled={deleting}>
				{deleting ? 'Deleting...' : 'Delete'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
