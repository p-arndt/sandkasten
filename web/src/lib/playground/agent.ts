/**
 * Sandkasten-backed agent for the playground.
 * Uses OpenAI Agents SDK with exec, read_file, write_file tools that call the Sandkasten API.
 * Supports OpenAI, Google (Gemini), and Google Vertex via AI SDK + agents-extensions.
 */

import { Agent, run, tool } from '@openai/agents';
import { z } from 'zod';
import { api } from '$lib/api';
import type { PlaygroundContext, PlaygroundSettings, SandkastenClientLike } from './types';
import type { RunContext } from '@openai/agents';
import { getModel } from './providers';
import { aisdk } from '@openai/agents-extensions';

function getSessionId(context: RunContext<PlaygroundContext> | undefined): string | null {
	return context?.context?.sessionId ?? null;
}

function getClient(context: RunContext<PlaygroundContext> | undefined): SandkastenClientLike {
	return context?.context?.sandkastenClient ?? api;
}

const execTool = tool({
	name: 'exec',
	description: `Execute a shell command in the sandbox. The sandbox is stateful: cd, environment variables, and background processes persist between calls. Use for running bash commands, python3, pip, etc.`,
	parameters: z.object({
		cmd: z.string().describe('Shell command to execute (e.g. "ls -la", "python3 script.py")'),
		timeout_ms: z.number().optional().default(30000).describe('Timeout in milliseconds')
	}),
	async execute(args, runContext) {
		const sessionId = getSessionId(runContext);
		if (!sessionId) return 'Error: No sandbox session. Start a session in the playground first.';
		const client = getClient(runContext);
		try {
			const result = await client.execCommand(sessionId, {
				cmd: args.cmd,
				timeout_ms: args.timeout_ms
			});
			const parts = [
				`exit_code=${result.exit_code}`,
				`cwd=${result.cwd}`
			];
			if (result.truncated) parts.push('(output truncated)');
			return parts.join('\n') + '\n---\n' + result.output;
		} catch (e) {
			return `Error: ${e instanceof Error ? e.message : String(e)}`;
		}
	}
});

const writeFileTool = tool({
	name: 'write_file',
	description: 'Write text content to a file in the sandbox workspace. Path can be relative to /workspace or absolute.',
	parameters: z.object({
		path: z.string().describe('File path (relative to /workspace or absolute)'),
		content: z.string().describe('File content')
	}),
	async execute(args, runContext) {
		const sessionId = getSessionId(runContext);
		if (!sessionId) return 'Error: No sandbox session. Start a session in the playground first.';
		const client = getClient(runContext);
		try {
			await client.writeFile(sessionId, { path: args.path, text: args.content });
			return `wrote ${args.path}`;
		} catch (e) {
			return `Error: ${e instanceof Error ? e.message : String(e)}`;
		}
	}
});

const readFileTool = tool({
	name: 'read_file',
	description: 'Read a file from the sandbox workspace. Path can be relative to /workspace or absolute.',
	parameters: z.object({
		path: z.string().describe('File path (relative to /workspace or absolute)'),
		max_bytes: z.number().optional().describe('Maximum bytes to read (optional)')
	}),
	async execute(args, runContext) {
		const sessionId = getSessionId(runContext);
		if (!sessionId) return 'Error: No sandbox session. Start a session in the playground first.';
		const client = getClient(runContext);
		try {
			const result = await client.readFile(sessionId, args.path, args.max_bytes);
			const decoder = new TextDecoder();
			const bytes = Uint8Array.from(
				atob(result.content_base64),
				(c) => c.charCodeAt(0)
			);
			const text = decoder.decode(bytes);
			return result.truncated ? text + '\n(truncated)' : text;
		} catch (e) {
			return `Error: ${e instanceof Error ? e.message : String(e)}`;
		}
	}
});

const AGENT_INSTRUCTIONS = `You are a helpful coding assistant with access to a Linux sandbox.

Available tools:
- exec(cmd, timeout_ms?): Run shell commands (bash, python3, pip, etc.). The sandbox is stateful.
- write_file(path, content): Write files in the workspace.
- read_file(path, max_bytes?): Read files from the workspace.

The sandbox has:
- Python 3 with pip and uv
- Development tools: git, curl, wget, jq
- Persistent /workspace directory
- Stateful shell (cd, env vars persist between commands)

Be helpful and thorough. Write clean code, run it to verify, and explain what you do.`;

/**
 * Create the playground agent using the selected provider/model from settings.
 * Call run() with context: { sessionId } and stream: true.
 */
export async function createPlaygroundAgent(settings: PlaygroundSettings) {
	const aiSdkModel = await getModel(settings);
	const model = aisdk(aiSdkModel);
	return new Agent<PlaygroundContext>({
		name: 'Sandbox Assistant',
		instructions: AGENT_INSTRUCTIONS,
		model,
		tools: [execTool, writeFileTool, readFileTool]
	});
}

export { run };
