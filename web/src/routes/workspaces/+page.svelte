<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Folder, RefreshCw, Trash2, Copy, HardDrive, FileText, FolderOpen, ChevronRight, ChevronLeft, Home } from '@lucide/svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { ScrollArea } from '$lib/components/ui/scroll-area';
	import { api } from '$lib/api';
	import { formatRelativeTime, formatTimestamp } from '$lib/utils/time';
	import { copyToClipboard } from '$lib/utils/clipboard';
	import type { Workspace, WorkspaceFileEntry } from '$lib/types';
	import { toast } from 'svelte-sonner';

	let workspaces = $state<Workspace[]>([]);
	let loading = $state(true);
	let refreshInterval: number;

	// Delete dialog state
	let deleteDialogOpen = $state(false);
	let workspaceToDelete = $state<Workspace | null>(null);
	let deleting = $state(false);

	// File explorer sheet
	let filesSheetOpen = $state(false);
	let filesWorkspace = $state<Workspace | null>(null);
	let currentPath = $state('');
	let fileEntries = $state<WorkspaceFileEntry[]>([]);
	let filesLoading = $state(false);
	let filesError = $state<string | null>(null);

	// Preview pane (right side of file explorer)
	let selectedFilePath = $state<string | null>(null);
	let previewContent = $state('');
	let previewTruncated = $state(false);
	let previewLoading = $state(false);
	let previewError = $state<string | null>(null);

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
		if (!workspaceToDelete?.id) return;

		deleting = true;
		try {
			await api.deleteWorkspace(workspaceToDelete.id);
			toast.success('Workspace deleted', {
				description: `Workspace ${workspaceToDelete.id?.substring(0, 12) ?? workspaceToDelete.id ?? 'unknown'}... has been removed`
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

	function openFileExplorer(workspace: Workspace) {
		filesWorkspace = workspace;
		currentPath = '';
		selectedFilePath = null;
		fileEntries = [];
		filesSheetOpen = true;
		filesError = null;
		previewContent = '';
		previewError = null;
		loadFileEntries();
	}

	async function loadFileEntries() {
		if (!filesWorkspace?.id) return;
		filesLoading = true;
		filesError = null;
		try {
			const res = await api.getWorkspaceFiles(filesWorkspace.id, currentPath);
			// Sort: directories first, then by name
			const entries = (res.entries || []).slice().sort((a, b) => {
				if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
				return a.name.localeCompare(b.name);
			});
			fileEntries = entries;
		} catch (err) {
			filesError = err instanceof Error ? err.message : 'Failed to load';
			fileEntries = [];
		} finally {
			filesLoading = false;
		}
	}

	function navigateTo(pathSegment: string) {
		currentPath = currentPath ? `${currentPath}/${pathSegment}` : pathSegment;
		selectedFilePath = null;
		loadFileEntries();
	}

	function goUp() {
		if (!currentPath) return;
		const parts = currentPath.split('/').filter(Boolean);
		parts.pop();
		currentPath = parts.join('/');
		selectedFilePath = null;
		loadFileEntries();
	}

	function selectFile(entry: WorkspaceFileEntry) {
		if (entry.is_dir) {
			navigateTo(entry.name);
			return;
		}
		const path = currentPath ? `${currentPath}/${entry.name}` : entry.name;
		selectedFilePath = path;
		previewContent = '';
		previewTruncated = false;
		previewError = null;
		previewLoading = true;
		loadFileContent(path);
	}

	async function loadFileContent(path: string) {
		if (!filesWorkspace?.id) return;
		try {
			const res = await api.readWorkspaceFile(filesWorkspace.id, path);
			previewTruncated = res.truncated;
			try {
				previewContent = atob(res.content_base64 || '');
			} catch {
				previewContent = res.content_base64 || '(empty)';
			}
			previewError = null;
		} catch (err) {
			previewError = err instanceof Error ? err.message : 'Failed to read file';
			previewContent = '';
		} finally {
			previewLoading = false;
		}
	}

	function isSelected(entry: WorkspaceFileEntry): boolean {
		if (entry.is_dir) return false;
		const path = currentPath ? `${currentPath}/${entry.name}` : entry.name;
		return selectedFilePath === path;
	}

	function pathBreadcrumbs(): string[] {
		if (!currentPath) return [];
		return currentPath.split('/').filter(Boolean);
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
										{#if workspace.id}
											<button
												onclick={() => copyId(workspace.id)}
												class="group flex items-center gap-2 hover:text-primary"
												title="Click to copy full ID"
											>
												{workspace.id.substring(0, 12)}...
												<Copy class="h-3 w-3 opacity-0 transition-opacity group-hover:opacity-100" />
											</button>
										{:else}
											<span class="text-muted-foreground">-</span>
										{/if}
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
										<div class="flex items-center justify-end gap-1">
											{#if workspace.id}
												<Button
													variant="outline"
													size="sm"
													title="Browse files"
													onclick={() => openFileExplorer(workspace)}
												>
													<FolderOpen class="mr-1.5 h-4 w-4" />
													Browse
												</Button>
											{/if}
											<Button
												variant="ghost"
												size="sm"
												onclick={() => openDeleteDialog(workspace)}
											>
												<Trash2 class="h-4 w-4 text-destructive" />
											</Button>
										</div>
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
					{workspaceToDelete?.id ? `${workspaceToDelete.id.substring(0, 12)}...` : '?'}
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

<!-- File Explorer Sheet: two-pane (list | preview). Override default sm:max-w-sm so the sheet is wide. -->
<Sheet.Root bind:open={filesSheetOpen}>
	<Sheet.Content class="flex h-full max-h-[90vh] w-full max-w-[95vw] flex-col gap-0 p-0 sm:max-w-[min(95vw,1280px)]">
		<Sheet.Header class="shrink-0 border-b px-4 py-3">
			<div class="flex items-center gap-3">
				<div class="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10">
					<FolderOpen class="h-5 w-5 text-primary" />
				</div>
				<div class="min-w-0 flex-1">
					<Sheet.Title class="text-lg">File explorer</Sheet.Title>
					<Sheet.Description class="mt-0.5 font-mono text-xs text-muted-foreground truncate" title={filesWorkspace?.id ?? ''}>
						{filesWorkspace?.id ?? '—'}
					</Sheet.Description>
				</div>
			</div>
		</Sheet.Header>

		<div class="flex min-h-0 flex-1">
			<!-- Left pane: breadcrumb + file list -->
			<div class="flex w-72 shrink-0 flex-col border-r">
				<div class="flex items-center gap-2 border-b bg-muted/30 px-2 py-2">
					{#if currentPath}
						<Button
							variant="ghost"
							size="sm"
							class="h-8 w-8 shrink-0 p-0"
							title="Parent folder"
							onclick={goUp}
						>
							<ChevronLeft class="h-4 w-4" />
						</Button>
					{/if}
					<div class="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto py-1">
						<button
							type="button"
							class="flex shrink-0 items-center gap-1.5 rounded-md bg-background px-2 py-1 text-sm font-medium shadow-sm ring-1 ring-border/50 hover:bg-muted/50"
							onclick={() => { currentPath = ''; selectedFilePath = null; loadFileEntries(); }}
							title="Root"
						>
							<Home class="h-4 w-4 text-muted-foreground" />
							<span>/</span>
						</button>
						{#each pathBreadcrumbs() as segment, i}
							<ChevronRight class="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
							<button
								type="button"
								class="shrink-0 rounded-md px-2 py-1 text-sm text-muted-foreground hover:bg-background hover:text-foreground hover:shadow-sm"
								onclick={() => {
									currentPath = pathBreadcrumbs().slice(0, i + 1).join('/');
									selectedFilePath = null;
									loadFileEntries();
								}}
							>
								{segment}
							</button>
						{/each}
					</div>
				</div>
				<div class="flex-1 min-h-0 overflow-hidden p-2">
					{#if filesError}
						<div class="flex flex-col items-center justify-center gap-3 rounded-lg border border-destructive/30 bg-destructive/5 py-8">
							<p class="text-center text-sm font-medium text-destructive">{filesError}</p>
							<Button variant="outline" size="sm" onclick={loadFileEntries}>Try again</Button>
						</div>
					{:else if filesLoading}
						<div class="space-y-2">
							{#each Array(6) as _}
								<Skeleton class="h-10 w-full rounded-lg" />
							{/each}
						</div>
					{:else}
						<ScrollArea class="h-full">
							<div class="space-y-0.5 py-1">
								{#each fileEntries as entry}
									<button
										type="button"
										class="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-sm transition-colors hover:bg-muted/80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {isSelected(entry)
											? 'bg-primary/10 text-primary hover:bg-primary/15'
											: ''}"
										onclick={() => selectFile(entry)}
									>
										{#if entry.is_dir}
											<Folder class="h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
										{:else}
											<FileText class="h-4 w-4 shrink-0 text-muted-foreground" />
										{/if}
										<span class="min-w-0 truncate">{entry.name}</span>
									</button>
								{/each}
								{#if !filesLoading && fileEntries.length === 0 && !filesError}
									<div class="py-8 text-center">
										<Folder class="mx-auto mb-2 h-8 w-8 text-muted-foreground" />
										<p class="text-xs text-muted-foreground">Empty folder</p>
									</div>
								{/if}
							</div>
						</ScrollArea>
					{/if}
				</div>
			</div>

			<!-- Right pane: file preview (min width so it's never a thin strip) -->
			<div class="flex min-w-[min(400px,60vw)] flex-1 flex-col bg-muted/20">
				{#if selectedFilePath}
					<div class="shrink-0 border-b bg-background px-4 py-2">
						<div class="flex items-center gap-2">
							<FileText class="h-4 w-4 shrink-0 text-muted-foreground" />
							<span class="truncate font-mono text-sm" title={selectedFilePath}>{selectedFilePath}</span>
							{#if previewTruncated}
								<span class="shrink-0 rounded border border-amber-500/30 bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:text-amber-400">truncated</span>
							{/if}
						</div>
					</div>
					<div class="flex-1 min-h-0 overflow-hidden">
						{#if previewLoading}
							<div class="space-y-2 p-4">
								{#each Array(12) as _}
									<Skeleton class="h-4 w-full rounded" />
								{/each}
							</div>
						{:else if previewError}
							<div class="flex flex-col items-center justify-center gap-3 p-8">
								<p class="text-sm font-medium text-destructive">{previewError}</p>
							</div>
						{:else}
							<ScrollArea class="h-full">
								<pre class="p-4 font-mono text-[13px] leading-relaxed whitespace-pre-wrap wrap-break-word">{previewContent}</pre>
							</ScrollArea>
						{/if}
					</div>
				{:else}
					<div class="flex flex-1 flex-col items-center justify-center gap-2 text-center text-muted-foreground">
						<FileText class="h-12 w-12 opacity-40" />
						<p class="text-sm font-medium">Select a file to view</p>
						<p class="text-xs">Click a file in the list to see its contents here</p>
					</div>
				{/if}
			</div>
		</div>
	</Sheet.Content>
</Sheet.Root>
