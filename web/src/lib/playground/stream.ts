/**
 * Consume OpenAI Agents SDK stream and yield normalized events for the UI.
 * Matches event shape from @openai/agents (run_item_stream_event with name/item, item.rawItem or item.raw_item).
 */

import type { StreamEventKind } from './types';

type StreamEvent = {
	type: string;
	name?: string;
	item?: {
		type?: string;
		id?: string;
		name?: string;
		arguments?: string;
		output?: unknown;
		raw_item?: { id?: string; name?: string; arguments?: string; call_id?: string };
		rawItem?: { id?: string; name?: string; arguments?: string; callId?: string };
	};
	data?: { type?: string; delta?: string };
};

function getItemFields(item: StreamEvent['item']) {
	// SDK may use camelCase (rawItem, callId) or snake_case (raw_item, call_id)
	const raw = item?.rawItem ?? item?.raw_item ?? item;
	const rawObj = raw as { callId?: string; call_id?: string; id?: string; name?: string; arguments?: string } | undefined;
	const itemObj = item as { call_id?: string; tool_call_id?: string; id?: string; name?: string; arguments?: string; output?: unknown } | undefined;
	const id =
		rawObj?.callId ??
		rawObj?.call_id ??
		rawObj?.id ??
		itemObj?.call_id ??
		itemObj?.tool_call_id ??
		itemObj?.id;
	return {
		id,
		name: rawObj?.name ?? itemObj?.name,
		arguments: rawObj?.arguments ?? itemObj?.arguments,
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
					// Only emit when we have a stable id so tool_result can match via toolCallMap
					const stableId = id ?? `tc-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
					let args: Record<string, unknown> = {};
					try {
						if (typeof argsStr === 'string') args = JSON.parse(argsStr) as Record<string, unknown>;
					} catch {
						// ignore
					}
					onEvent({
						type: 'tool_call',
						id: stableId,
						name: name ?? (args?.cmd ? 'exec' : args?.path ? 'read_file' : 'tool'),
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
