<script lang="ts">
	import { marked } from 'marked';
	import { Badge } from '$lib/components/ui/badge';
	import { Wrench, Loader2, CircleCheck } from '@lucide/svelte';
	import type { ToolCallDisplay } from '$lib/playground/types';

	let { toolCalls = [], content = '', streaming = false } = $props();

	function renderMarkdown(text: string): string {
		if (!text?.trim()) return '';
		return marked.parse(text, { async: false, breaks: true }) as string;
	}

	/** One-line summary for display (exec → command, read_file/write_file → path). */
	function toolSummary(tc: ToolCallDisplay): string {
		if (typeof tc.args?.cmd === 'string') return tc.args.cmd;
		if (typeof tc.args?.path === 'string') return tc.name === 'write_file' ? `Write: ${tc.args.path}` : tc.args.path;
		if (Object.keys(tc.args ?? {}).length > 0) return JSON.stringify(tc.args);
		return '';
	}
</script>

<div class="space-y-2">
	{#if toolCalls.length > 0}
		<div class="mb-3 space-y-2">
			{#each toolCalls as tc}
				<div
					class="rounded-lg border bg-card/50 p-3 font-mono text-xs shadow-sm {tc.status === 'running'
						? 'border-amber-500/50 ring-1 ring-amber-500/10'
						: 'border-border/50'}"
				>
					<div class="flex flex-wrap items-center gap-2">
						<Wrench class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
						<Badge variant="outline" class="font-mono text-[10px]">{tc.name}</Badge>
						{#if tc.status === 'running'}
							<Loader2 class="h-3 w-3 shrink-0 animate-spin text-amber-500" />
						{:else}
							<CircleCheck class="h-3 w-3 shrink-0 text-green-600 dark:text-green-400" />
						{/if}
					</div>
					{#if toolSummary(tc)}
						<code class="mt-1.5 block truncate rounded bg-muted/30 px-1.5 py-0.5 text-[11px] text-muted-foreground">
							> {toolSummary(tc)}
						</code>
					{/if}
					<pre class="mt-2 max-h-40 overflow-auto whitespace-pre-wrap break-all rounded bg-background/80 p-2 text-[11px] text-foreground">{tc.output ?? (tc.status === 'running' ? '' : '—')}</pre>
				</div>
			{/each}
		</div>
	{/if}
	{#if content.trim()}
		<div class="rounded-lg border border-border/50 bg-muted/80 px-4 py-3">
			<div class="prose prose-sm dark:prose-invert max-w-none text-foreground">
				{@html renderMarkdown(content)}
			</div>
		</div>
	{/if}
</div>
