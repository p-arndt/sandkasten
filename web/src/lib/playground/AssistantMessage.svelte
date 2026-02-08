<script lang="ts">
	import { marked } from 'marked';
	import { Badge } from '$lib/components/ui/badge';
	import { Wrench, Loader2 } from '@lucide/svelte';
	import type { ToolCallDisplay } from '$lib/playground/types';

	let { toolCalls = [], content = '', streaming = false } = $props();

	function renderMarkdown(text: string): string {
		if (!text?.trim()) return '';
		return marked.parse(text, { async: false, breaks: true }) as string;
	}
</script>

<div class="space-y-2">
	{#if toolCalls.length > 0}
		<div class="mb-3 space-y-2">
			{#each toolCalls as tc}
				<div
					class="rounded-md border bg-muted/50 p-3 font-mono text-xs {tc.status === 'running' ? 'border-amber-500/50' : 'border-green-500/30'}"
				>
					<div class="mb-1 flex items-center gap-2">
						<Wrench class="h-3.5 w-3.5 text-muted-foreground" />
						<Badge variant="outline" class="font-mono">{tc.name}</Badge>
						<Loader2 class="h-3 w-3 animate-spin {tc.status !== 'running' ? 'invisible' : ''}" />
					</div>
					<pre class="mb-2 overflow-x-auto text-muted-foreground">{Object.keys(tc.args).length > 0 ? JSON.stringify(tc.args, null, 2) : ''}</pre>
					<pre class="max-h-40 overflow-auto whitespace-pre-wrap break-all rounded bg-background/80 p-2 text-[11px]">{tc.output ?? ''}</pre>
				</div>
			{/each}
		</div>
	{/if}
	{#if content.trim()}
		<div class="rounded-lg bg-muted/80 px-4 py-3 border border-border/50">
			<div class="prose prose-sm dark:prose-invert max-w-none text-foreground">
				{@html renderMarkdown(content)}
			</div>
		</div>
	{/if}
</div>
