/**
 * Consume OpenAI Agents SDK stream and yield normalized events for the UI.
 */

import type { StreamEventKind } from './types';

type StreamEvent = {
	type: string;
	item?: { type?: string; id?: string; name?: string; arguments?: string; output?: unknown; raw_item?: { id?: string; name?: string; arguments?: string } };
	data?: { type?: string; delta?: string };
	name?: string;
};

function getItemFields(item: StreamEvent['item']) {
	const raw = item?.raw_item ?? item;
	return {
		id: raw?.id ?? item?.id,
		name: raw?.name ?? item?.name,
		arguments: raw?.arguments ?? item?.arguments,
		output: item?.output
	};
}

/**
 * Consume the stream from run(agent, input, { stream: true }) and call onEvent for each UI-relevant event.
 * Resolves when the run is complete; rejects on stream error.
 */
export async function consumeAgentStream(
	stream: AsyncIterable<StreamEvent> & { completed?: Promise<unknown> },
	onEvent: (event: StreamEventKind) => void
): Promise<void> {
	try {
		for await (const event of stream) {
			if (event.type === 'raw_model_stream_event' && event.data?.type === 'output_text_delta' && event.data.delta) {
				onEvent({ type: 'text_delta', delta: event.data.delta });
			} else if (event.type === 'run_item_stream_event' && event.item) {
				const { id, name, arguments: argsStr, output } = getItemFields(event.item);
				const itemType = event.item.type ?? event.name;
				const isToolCall =
					itemType === 'tool_call_item' ||
					event.name === 'tool_called' ||
					(itemType === 'function_call_item' && name);
				const isToolOutput =
					itemType === 'tool_call_output_item' ||
					event.name === 'tool_output' ||
					itemType === 'function_call_result_item';

				if (isToolCall && !isToolOutput) {
					let args: Record<string, unknown> = {};
					try {
						if (typeof argsStr === 'string') args = JSON.parse(argsStr) as Record<string, unknown>;
					} catch {
						// ignore
					}
					onEvent({
						type: 'tool_call',
						id: id ?? `tc-${Date.now()}`,
						name: name ?? 'unknown',
						args
					});
				} else if (isToolOutput || output !== undefined) {
					const out = output;
					const text =
						typeof out === 'string'
							? out
							: out && typeof (out as { content?: string }).content === 'string'
								? (out as { content: string }).content
								: JSON.stringify(out ?? '');
					onEvent({ type: 'tool_result', id: id ?? '', output: text });
				}
			}
		}
		onEvent({ type: 'done' });
	} catch (err) {
		onEvent({
			type: 'error',
			message: err instanceof Error ? err.message : String(err)
		});
	}
	if (stream.completed) {
		await stream.completed;
	}
}
