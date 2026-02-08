<script lang="ts">
	import { onMount } from 'svelte';
	import { Settings as SettingsIcon, Save, RefreshCw, CheckCircle, AlertTriangle } from '@lucide/svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import * as Alert from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { api } from '$lib/api';
	import { toast } from 'svelte-sonner';

	let configContent = $state('');
	let configPath = $state('');
	let loading = $state(true);
	let saving = $state(false);
	let validating = $state(false);
	let referenceOpen = $state(false);

	async function loadConfig() {
		loading = true;
		try {
			const response = await api.getConfig();
			configContent = response.content;
			configPath = response.path;
			loading = false;
		} catch (err) {
			toast.error('Failed to load configuration', {
				description: err instanceof Error ? err.message : 'Unknown error'
			});
			loading = false;
		}
	}

	async function saveConfig() {
		saving = true;
		try {
			await api.saveConfig(configContent);
			toast.success('Configuration saved', {
				description: 'Restart the daemon to apply changes'
			});
		} catch (err) {
			toast.error('Failed to save configuration', {
				description: err instanceof Error ? err.message : 'Unknown error'
			});
		} finally {
			saving = false;
		}
	}

	async function validateConfig() {
		validating = true;
		try {
			const result = await api.validateConfig(configContent);
			if (result.valid) {
				toast.success('Configuration is valid', {
					description: 'YAML syntax is correct'
				});
			} else {
				toast.error('Invalid configuration', {
					description: result.error || 'YAML syntax error'
				});
			}
		} catch (err) {
			toast.error('Validation failed', {
				description: err instanceof Error ? err.message : 'Unknown error'
			});
		} finally {
			validating = false;
		}
	}

	onMount(() => {
		loadConfig();
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-3xl font-bold tracking-tight">Settings</h1>
			<p class="text-muted-foreground">Configure sandkasten daemon</p>
		</div>
	</div>

	<!-- Warning Banner -->
	<Alert.Root variant="destructive">
		<AlertTriangle class="h-4 w-4" />
		<Alert.Title>Daemon Restart Required</Alert.Title>
		<Alert.Description>
			Changes to the configuration file require restarting the Sandkasten daemon to take effect.
			Invalid YAML will prevent the daemon from starting.
		</Alert.Description>
	</Alert.Root>

	<!-- Config Editor -->
	<Card.Root>
		<Card.Header>
			<Card.Title>Configuration Editor</Card.Title>
			<Card.Description>
				{#if configPath}
					Editing: <code class="rounded bg-muted px-1 py-0.5 text-xs">{configPath}</code>
				{:else}
					Edit sandkasten.yaml
				{/if}
			</Card.Description>
		</Card.Header>
		<Card.Content class="space-y-4">
			{#if loading}
				<Skeleton class="h-96 w-full" />
			{:else}
				<Textarea
					bind:value={configContent}
					class="min-h-[500px] font-mono text-sm"
					placeholder="# Sandkasten configuration..."
					spellcheck="false"
				/>
			{/if}
			
			<div class="flex gap-2">
				<Button onclick={saveConfig} disabled={saving || loading}>
					<Save class="mr-2 h-4 w-4" />
					{saving ? 'Saving...' : 'Save Configuration'}
				</Button>
				<Button variant="outline" onclick={loadConfig} disabled={loading}>
					<RefreshCw class="mr-2 h-4 w-4" />
					Reload from File
				</Button>
				<Button variant="outline" onclick={validateConfig} disabled={validating || loading}>
					<CheckCircle class="mr-2 h-4 w-4" />
					{validating ? 'Validating...' : 'Validate YAML'}
				</Button>
			</div>
		</Card.Content>
	</Card.Root>

	<!-- Configuration Reference -->
	<Collapsible.Root bind:open={referenceOpen}>
		<Card.Root>
			<Card.Header>
				<div class="flex items-center justify-between">
					<div>
						<Card.Title>Configuration Reference</Card.Title>
						<Card.Description>Available configuration options</Card.Description>
					</div>
					<Collapsible.Trigger asChild let:builder>
						<Button builders={[builder]} variant="ghost" size="sm">
							{referenceOpen ? 'Hide' : 'Show'}
						</Button>
					</Collapsible.Trigger>
				</div>
			</Card.Header>
			<Collapsible.Content>
				<Card.Content class="space-y-4 text-sm">
					<div>
						<h4 class="mb-1 font-semibold">listen:</h4>
						<p class="text-muted-foreground">Host and port to bind (e.g., "127.0.0.1:8080")</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">api_key:</h4>
						<p class="text-muted-foreground">API key for authentication (Bearer token)</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">default_image:</h4>
						<p class="text-muted-foreground">Default Docker image for sessions</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">allowed_images:</h4>
						<p class="text-muted-foreground">List of allowed Docker images</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">db_path:</h4>
						<p class="text-muted-foreground">Path to SQLite database file</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">session_ttl_seconds:</h4>
						<p class="text-muted-foreground">Default session lifetime in seconds</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">defaults.cpu_limit:</h4>
						<p class="text-muted-foreground">CPU limit (e.g., 1.0 = 1 core)</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">defaults.mem_limit_mb:</h4>
						<p class="text-muted-foreground">Memory limit in megabytes</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">defaults.pids_limit:</h4>
						<p class="text-muted-foreground">Process limit</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">defaults.max_exec_timeout_ms:</h4>
						<p class="text-muted-foreground">Maximum exec timeout in milliseconds</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">defaults.network_mode:</h4>
						<p class="text-muted-foreground">Network mode ("none", "bridge", "host")</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">defaults.readonly_rootfs:</h4>
						<p class="text-muted-foreground">Whether to make rootfs read-only (true/false)</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">pool.enabled:</h4>
						<p class="text-muted-foreground">Enable container pool for fast session creation</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">pool.images:</h4>
						<p class="text-muted-foreground">Map of image names to pool sizes (e.g., sandbox-runtime:python: 3)</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">workspace.enabled:</h4>
						<p class="text-muted-foreground">Enable persistent workspace volumes</p>
					</div>
					<div>
						<h4 class="mb-1 font-semibold">workspace.persist_by_default:</h4>
						<p class="text-muted-foreground">Create persistent workspaces by default</p>
					</div>
				</Card.Content>
			</Collapsible.Content>
		</Card.Root>
	</Collapsible.Root>
</div>
