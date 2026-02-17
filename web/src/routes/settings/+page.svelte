<script lang="ts">
	import { onMount } from 'svelte';
	import { Settings as SettingsIcon, Save, RefreshCw, CheckCircle, AlertTriangle, Key } from '@lucide/svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import * as Alert from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Switch } from '$lib/components/ui/switch';
	import { Label } from '$lib/components/ui/label';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { api } from '$lib/api';
	import { getStoredApiKey, setStoredApiKey } from '$lib/api-key';
	import { getStoredPlaygroundSettings, setStoredPlaygroundSettings, getEffectivePlaygroundSettings, sessionOnlyKeys } from '$lib/playground/settings';
	import type { PlaygroundSettings, ProviderId } from '$lib/playground/types';
	import { PROVIDER_LABELS, DEFAULT_MODELS } from '$lib/playground/types';
	import { toast } from 'svelte-sonner';

	let configContent = $state('');
	let configPath = $state('');
	let loading = $state(true);
	let saving = $state(false);
	let validating = $state(false);
	let referenceOpen = $state(false);

	// API key (masked in UI when loaded from storage)
	let apiKeyInput = $state('');
	let apiKeySaved = $state(false);

	// Playground provider/model/keys
	let playgroundSettings = $state<PlaygroundSettings>(getStoredPlaygroundSettings());
	let loadingPlaygroundConfig = $state(false);
	let loadPlaygroundConfigError = $state('');
	let savingPlaygroundToBackend = $state(false);
	let savePlaygroundToBackendError = $state('');

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

	function saveApiKey() {
		const key = apiKeyInput.trim() || null;
		setStoredApiKey(key);
		api.setAPIKey(key);
		apiKeyInput = '';
		apiKeySaved = !!key;
		toast.success(key ? 'API key saved' : 'API key cleared', {
			description: key ? 'Key is used for all API requests.' : 'Requests will no longer send a Bearer token.'
		});
	}

	function savePlaygroundSettings() {
		setStoredPlaygroundSettings(playgroundSettings);
		if (!playgroundSettings.persistProviderKeys) {
			sessionOnlyKeys.set({
				openaiApiKey: playgroundSettings.openaiApiKey || undefined,
				googleApiKey: playgroundSettings.googleApiKey || undefined,
				googleVertexApiKey: playgroundSettings.googleVertexApiKey || undefined
			});
		} else {
			sessionOnlyKeys.set({});
		}
		toast.success('Playground settings saved');
	}

	function setPlaygroundProvider(provider: ProviderId) {
		playgroundSettings = {
			...playgroundSettings,
			provider,
			model: playgroundSettings.model?.trim() || DEFAULT_MODELS[provider]
		};
	}

	async function loadPlaygroundConfigFromBackend() {
		loadPlaygroundConfigError = '';
		savePlaygroundToBackendError = '';
		loadingPlaygroundConfig = true;
		try {
			const res = await api.getPlaygroundConfig();
			const provider = (res.provider as ProviderId) || 'openai';
			const model = res.model?.trim() || DEFAULT_MODELS[provider];
			playgroundSettings = {
				...playgroundSettings,
				provider,
				model,
				vertexProject: res.vertexProject ?? playgroundSettings.vertexProject,
				vertexLocation: res.vertexLocation ?? playgroundSettings.vertexLocation,
				openaiApiKey: '',
				googleApiKey: '',
				googleVertexApiKey: ''
			};
			setStoredPlaygroundSettings(playgroundSettings);
			sessionOnlyKeys.set({
				openaiApiKey: res.openaiApiKey || undefined,
				googleApiKey: res.googleApiKey || undefined,
				googleVertexApiKey: res.googleVertexApiKey || undefined
			});
			toast.success('Provider keys loaded from backend', {
				description: 'Keys are in memory only. Use Playground now.'
			});
		} catch (err) {
			loadPlaygroundConfigError = err instanceof Error ? err.message : 'Failed to load';
			toast.error('Failed to load from backend', {
				description: loadPlaygroundConfigError
			});
		} finally {
			loadingPlaygroundConfig = false;
		}
	}

	async function savePlaygroundConfigToBackend() {
		savePlaygroundToBackendError = '';
		loadPlaygroundConfigError = '';
		savingPlaygroundToBackend = true;
		try {
			const effective = getEffectivePlaygroundSettings(playgroundSettings, $sessionOnlyKeys);
			await api.savePlaygroundConfig({
				provider: effective.provider,
				model: effective.model,
				openaiApiKey: effective.openaiApiKey,
				googleApiKey: effective.googleApiKey,
				googleVertexApiKey: effective.googleVertexApiKey,
				vertexProject: effective.vertexProject,
				vertexLocation: effective.vertexLocation
			});
			toast.success('Playground config saved to backend', {
				description: 'Stored in JSON file (playground_config_path).'
			});
		} catch (err) {
			savePlaygroundToBackendError = err instanceof Error ? err.message : 'Failed to save';
			toast.error('Failed to save to backend', {
				description: savePlaygroundToBackendError
			});
		} finally {
			savingPlaygroundToBackend = false;
		}
	}

	onMount(() => {
		loadConfig();
		if (getStoredApiKey()) apiKeySaved = true;
		playgroundSettings = getStoredPlaygroundSettings();
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-3xl font-bold tracking-tight">Settings</h1>
			<p class="text-muted-foreground">Configure sandkasten daemon</p>
		</div>
	</div>

	<!-- API Key -->
	<Card.Root>
		<Card.Header>
			<Card.Title class="flex items-center gap-2">
				<Key class="h-5 w-5" />
				API Key
			</Card.Title>
			<Card.Description>
				Dashboard uses this key to authenticate with the Sandkasten daemon. Use the same value as <code class="rounded bg-muted px-1 py-0.5 text-xs">api_key</code> in your daemon config. You can also set it from the banner at the top when not connected.
			</Card.Description>
		</Card.Header>
		<Card.Content class="space-y-4">
			<div class="flex gap-2">
				<Input
					bind:value={apiKeyInput}
					type="password"
					placeholder={apiKeySaved ? '••••••••••••' : 'Enter API key'}
					class="max-w-md font-mono"
					autocomplete="off"
				/>
				<Button onclick={saveApiKey}>
					<Save class="mr-2 h-4 w-4" />
					Save Key
				</Button>
			</div>
			{#if apiKeySaved && !apiKeyInput}
				<p class="text-xs text-muted-foreground">A key is stored. Enter a new value and save to replace, or save with an empty field to clear.</p>
			{/if}
		</Card.Content>
	</Card.Root>

	<!-- Playground (model provider: OpenAI, Google, Google Vertex) -->
	<Card.Root>
		<Card.Header>
			<Card.Title class="flex items-center gap-2">
				<Key class="h-5 w-5" />
				Playground (model provider)
			</Card.Title>
			<Card.Description>
				Provider and API key for the Playground coding agent. Choose OpenAI, Google (Gemini), or Google Vertex AI. Or load keys from the backend (daemon reads /config/config.json); keys are kept in memory only.
			</Card.Description>
		</Card.Header>
		<Card.Content class="space-y-4">
			<div class="flex flex-col gap-3 rounded-lg border p-4">
				<p class="font-medium">Backend config file</p>
				<p class="text-sm text-muted-foreground">Daemon reads/writes a JSON file (playground_config_path). Load fills keys in memory only; Save writes current provider and API keys to the file. File is created if missing.</p>
				<div class="flex flex-wrap items-center gap-2">
					<Button variant="outline" onclick={loadPlaygroundConfigFromBackend} disabled={loadingPlaygroundConfig}>
						{loadingPlaygroundConfig ? 'Loading…' : 'Load from backend'}
					</Button>
					<Button variant="outline" onclick={savePlaygroundConfigToBackend} disabled={savingPlaygroundToBackend}>
						{savingPlaygroundToBackend ? 'Saving…' : 'Save to backend'}
					</Button>
				</div>
				{#if loadPlaygroundConfigError || savePlaygroundToBackendError}
					<p class="text-sm text-destructive">{loadPlaygroundConfigError || savePlaygroundToBackendError}</p>
				{/if}
			</div>
			<div class="flex flex-wrap items-center gap-4">
				<div>
					<label for="pg-provider" class="mb-1 block text-sm font-medium">Provider</label>
					<select
						id="pg-provider"
						class="rounded-md border bg-background px-3 py-2 text-sm"
						value={playgroundSettings.provider}
						onchange={(e) => setPlaygroundProvider((e.currentTarget.value as ProviderId))}
					>
						{#each Object.entries(PROVIDER_LABELS) as [id, label]}
							<option value={id}>{label}</option>
						{/each}
					</select>
				</div>
				<div class="min-w-[200px]">
					<label for="pg-model" class="mb-1 block text-sm font-medium">Model</label>
					<Input
						id="pg-model"
						bind:value={playgroundSettings.model}
						placeholder={DEFAULT_MODELS[playgroundSettings.provider]}
						class="font-mono"
					/>
				</div>
			</div>
			{#if playgroundSettings.provider === 'openai'}
				<div>
					<label for="pg-openai-key" class="mb-1 block text-sm font-medium">OpenAI API key</label>
					<Input
						id="pg-openai-key"
						bind:value={playgroundSettings.openaiApiKey}
						type="password"
						placeholder="sk-..."
						class="max-w-md font-mono"
						autocomplete="off"
					/>
				</div>
			{:else if playgroundSettings.provider === 'google'}
				<div>
					<label for="pg-google-key" class="mb-1 block text-sm font-medium">Google API key (Gemini)</label>
					<Input
						id="pg-google-key"
						bind:value={playgroundSettings.googleApiKey}
						type="password"
						placeholder="AIza..."
						class="max-w-md font-mono"
						autocomplete="off"
					/>
				</div>
			{:else}
				<div class="space-y-3">
					<div>
						<label for="pg-vertex-key" class="mb-1 block text-sm font-medium">Vertex API key or service account JSON</label>
						<Input
							id="pg-vertex-key"
							bind:value={playgroundSettings.googleVertexApiKey}
							type="password"
							placeholder="Express API key or paste JSON (client_email, private_key)"
							class="max-w-md font-mono text-xs"
							autocomplete="off"
						/>
					</div>
					<div class="flex gap-4">
						<div>
							<label for="pg-vertex-project" class="mb-1 block text-sm font-medium">Project ID</label>
							<Input
								id="pg-vertex-project"
								bind:value={playgroundSettings.vertexProject}
								placeholder="my-gcp-project"
								class="font-mono"
							/>
						</div>
						<div>
							<label for="pg-vertex-location" class="mb-1 block text-sm font-medium">Location</label>
							<Input
								id="pg-vertex-location"
								bind:value={playgroundSettings.vertexLocation}
								placeholder="us-central1"
								class="font-mono"
							/>
						</div>
					</div>
					<p class="text-xs text-muted-foreground">Express mode: API key only, leave project empty. Project mode: set project + location and paste service account JSON in the key field.</p>
				</div>
			{/if}
			<div class="flex items-center justify-between rounded-lg border p-4">
				<div class="space-y-0.5">
					<Label for="pg-persist-keys" class="text-base">Save API keys in browser</Label>
					<p class="text-sm text-muted-foreground">When off, keys are kept only in memory until you leave the page. More secure; you re-enter keys after refresh.</p>
				</div>
				<Switch id="pg-persist-keys" bind:checked={playgroundSettings.persistProviderKeys} />
			</div>
			<div class="rounded-lg border p-4 space-y-4">
				<h4 class="font-medium">Session options</h4>
				<div class="grid gap-4 sm:grid-cols-2">
					<div>
						<label for="pg-session-image" class="mb-1 block text-sm font-medium">Image</label>
						<Input
							id="pg-session-image"
							bind:value={playgroundSettings.sessionImage}
							placeholder="sandbox-runtime:python"
							class="font-mono"
						/>
					</div>
					<div>
						<label for="pg-session-ttl" class="mb-1 block text-sm font-medium">TTL (seconds)</label>
						<Input
							id="pg-session-ttl"
							type="number"
							min="60"
							bind:value={playgroundSettings.sessionTtlSeconds}
							class="font-mono"
						/>
					</div>
				</div>
				<div class="flex items-center justify-between rounded-md border p-3">
					<div class="space-y-0.5">
						<Label for="pg-workspace-enabled" class="text-sm">Use persistent workspace</Label>
						<p class="text-xs text-muted-foreground">Files survive session end. Requires daemon <code class="rounded bg-muted px-1">workspace.enabled: true</code>.</p>
					</div>
					<Switch id="pg-workspace-enabled" bind:checked={playgroundSettings.workspaceEnabled} />
				</div>
				{#if playgroundSettings.workspaceEnabled}
					<div>
						<label for="pg-workspace-id" class="mb-1 block text-sm font-medium">Workspace ID</label>
						<Input
							id="pg-workspace-id"
							bind:value={playgroundSettings.workspaceId}
							placeholder="e.g. playground-default"
							class="font-mono max-w-md"
						/>
					</div>
				{/if}
			</div>
			<Button onclick={savePlaygroundSettings}>
				<Save class="mr-2 h-4 w-4" />
				Save Playground settings
			</Button>
		</Card.Content>
	</Card.Root>

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
					<Collapsible.Trigger >
							{referenceOpen ? 'Hide' : 'Show'}
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
