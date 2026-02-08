<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Activity, Box, CheckCircle, XCircle, Clock } from '@lucide/svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Progress } from '$lib/components/ui/progress';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { api } from '$lib/api';
	import { formatRelativeTime, formatTimestamp, formatUptime } from '$lib/utils/time';
	import type { StatusResponse, SessionInfo } from '$lib/types';
	import { toast } from 'svelte-sonner';

	let status = $state<StatusResponse | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let refreshInterval: number;
	let uptime = $state(0);
	let uptimeInterval: number;

	async function loadStatus() {
		try {
			error = null;
			status = await api.getStatus();
			if (status) {
				uptime = status.uptime_seconds;
			}
			loading = false;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load status';
			loading = false;
			toast.error('Failed to load status', {
				description: error
			});
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

	onMount(() => {
		loadStatus();
		refreshInterval = setInterval(loadStatus, 5000) as unknown as number;
		uptimeInterval = setInterval(() => {
			uptime++;
		}, 1000) as unknown as number;
	});

	onDestroy(() => {
		if (refreshInterval) clearInterval(refreshInterval);
		if (uptimeInterval) clearInterval(uptimeInterval);
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-3xl font-bold tracking-tight">Overview</h1>
			<p class="text-muted-foreground">Monitor your sandbox runtime environment</p>
		</div>
		<Button onclick={loadStatus} variant="outline" size="sm">
			<Activity class="mr-2 h-4 w-4" />
			Refresh
		</Button>
	</div>

	<!-- Stats Cards -->
	<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
		{#if loading}
			{#each Array(4) as _}
				<Card.Root>
					<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-2">
						<Skeleton class="h-4 w-20" />
					</Card.Header>
					<Card.Content>
						<Skeleton class="h-8 w-16" />
					</Card.Content>
				</Card.Root>
			{/each}
		{:else if status}
			<Card.Root>
				<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-2">
					<Card.Title class="text-sm font-medium">Total Sessions</Card.Title>
					<Box class="h-4 w-4 text-muted-foreground" />
				</Card.Header>
				<Card.Content>
					<div class="text-2xl font-bold">{status.total_sessions}</div>
					<p class="text-xs text-muted-foreground">All sessions created</p>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-2">
					<Card.Title class="text-sm font-medium">Active</Card.Title>
					<CheckCircle class="h-4 w-4 text-green-600" />
				</Card.Header>
				<Card.Content>
					<div class="text-2xl font-bold text-green-600">{status.active_sessions}</div>
					<p class="text-xs text-muted-foreground">Currently running</p>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-2">
					<Card.Title class="text-sm font-medium">Expired</Card.Title>
					<XCircle class="h-4 w-4 text-yellow-600" />
				</Card.Header>
				<Card.Content>
					<div class="text-2xl font-bold text-yellow-600">{status.expired_sessions}</div>
					<p class="text-xs text-muted-foreground">Past TTL</p>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-2">
					<Card.Title class="text-sm font-medium">Uptime</Card.Title>
					<Clock class="h-4 w-4 text-muted-foreground" />
				</Card.Header>
				<Card.Content>
					<div class="text-2xl font-bold">{formatUptime(uptime)}</div>
					<p class="text-xs text-muted-foreground">Daemon running</p>
				</Card.Content>
			</Card.Root>
		{/if}
	</div>

	<!-- Pool Status -->
	{#if status?.pool?.enabled && Object.keys(status.pool.images).length > 0}
		<Card.Root>
			<Card.Header>
				<Card.Title>Container Pool Status</Card.Title>
				<Card.Description>Pre-warmed containers ready for instant use</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-4">
				{#each Object.entries(status.pool.images) as [image, imageStatus]}
					<div class="space-y-2">
						<div class="flex items-center justify-between text-sm">
							<span class="font-medium">{image}</span>
							<span class="text-muted-foreground">
								{imageStatus.current} / {imageStatus.target}
							</span>
						</div>
						<Progress value={(imageStatus.current / imageStatus.target) * 100} />
					</div>
				{/each}
			</Card.Content>
		</Card.Root>
	{/if}

	<!-- Recent Sessions -->
	<Card.Root>
		<Card.Header>
			<Card.Title>Recent Sessions</Card.Title>
			<Card.Description>Last 5 sessions created</Card.Description>
		</Card.Header>
		<Card.Content>
			{#if loading}
				<div class="space-y-2">
					{#each Array(3) as _}
						<Skeleton class="h-12 w-full" />
					{/each}
				</div>
			{:else if status && status.sessions.length > 0}
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>ID</Table.Head>
							<Table.Head>Image</Table.Head>
							<Table.Head>Status</Table.Head>
							<Table.Head>Created</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each status.sessions.slice(0, 5) as session}
							<Table.Row>
								<Table.Cell class="font-mono text-xs">{session.id.substring(0, 12)}...</Table.Cell>
								<Table.Cell>{session.image}</Table.Cell>
								<Table.Cell>
									<Badge variant={getStatusColor(session.status)}>{session.status}</Badge>
								</Table.Cell>
								<Table.Cell class="text-muted-foreground" title={formatTimestamp(session.created_at)}>
									{formatRelativeTime(session.created_at)}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			{:else}
				<div class="flex flex-col items-center justify-center py-8 text-center">
					<Box class="mb-2 h-8 w-8 text-muted-foreground" />
					<p class="text-sm text-muted-foreground">No sessions yet</p>
				</div>
			{/if}
		</Card.Content>
	</Card.Root>
</div>
