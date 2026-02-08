<script lang="ts">
  import { onMount, tick } from "svelte";
  import {
    Play,
    Square,
    Terminal,
    User,
    Bot,
    Loader2,
    Settings,
  } from "@lucide/svelte";
  import * as Card from "$lib/components/ui/card";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Badge } from "$lib/components/ui/badge";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { api } from "$lib/api";
  import { createPlaygroundAgent, run } from "$lib/playground/agent";
  import { getStoredPlaygroundSettings } from "$lib/playground/settings";
  import { hasRequiredConfig } from "$lib/playground/providers";
  import type { PlaygroundSettings } from "$lib/playground/types";
  import { PROVIDER_LABELS } from "$lib/playground/types";
  import AssistantMessage from "$lib/playground/AssistantMessage.svelte";
  import { consumeAgentStream } from "$lib/playground/stream";
  import type { ChatMessage, ToolCallDisplay } from "$lib/playground/types";
  import { toast } from "svelte-sonner";

  const DEFAULT_IMAGE = "sandbox-runtime:python";

  let sessionId = $state<string | null>(null);
  let sessionLoading = $state(false);
  let messages = $state<ChatMessage[]>([]);
  let inputText = $state("");
  let sending = $state(false);
  let messagesEndEl: HTMLDivElement;
  let settings = $state<PlaygroundSettings>(getStoredPlaygroundSettings());
  let showSettingsPrompt = $state(false);

  function addMessage(
    role: ChatMessage["role"],
    content: string,
    opts?: { toolCalls?: ToolCallDisplay[]; streaming?: boolean },
  ) {
    const id = `msg-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
    messages = [...messages, { id, role, content, ...opts }];
    return id;
  }

  function updateMessage(id: string, updates: Partial<ChatMessage>) {
    messages = messages.map((m) => (m.id === id ? { ...m, ...updates } : m));
  }

  function updateToolCall(
    messageId: string,
    toolId: string,
    updates: Partial<ToolCallDisplay>,
  ) {
    messages = messages.map((m) => {
      if (m.id !== messageId || !m.toolCalls) return m;
      return {
        ...m,
        toolCalls: m.toolCalls.map((tc) =>
          tc.id === toolId ? { ...tc, ...updates } : tc,
        ),
      };
    });
  }

  async function ensureSession(): Promise<string | null> {
    if (sessionId) return sessionId;
    sessionLoading = true;
    try {
      const session = await api.createSession({ image: DEFAULT_IMAGE });
      sessionId = session.id;
      toast.success("Sandbox session started", {
        description: `Session ${session.id.slice(0, 8)}...`,
      });
      return session.id;
    } catch (err) {
      toast.error("Failed to start session", {
        description: err instanceof Error ? err.message : "Unknown error",
      });
      return null;
    } finally {
      sessionLoading = false;
    }
  }

  async function endSession() {
    if (!sessionId) return;
    try {
      await api.destroySession(sessionId);
      toast.success("Session ended");
    } catch (err) {
      toast.error("Failed to end session", {
        description: err instanceof Error ? err.message : "Unknown error",
      });
    }
    sessionId = null;
  }

  async function sendMessage() {
    const text = inputText.trim();
    if (!text || sending) return;

    if (!hasRequiredConfig(settings)) {
      showSettingsPrompt = true;
      toast.error("API key required", {
        description: "Set provider and API key in Settings.",
      });
      return;
    }
    showSettingsPrompt = false;

    const sid = await ensureSession();
    if (!sid) return;

    inputText = "";
    const userMsgId = addMessage("user", text);
    const assistantMsgId = addMessage("assistant", "", {
      streaming: true,
      toolCalls: [],
    });
    sending = true;

    await tick();
    messagesEndEl?.scrollIntoView({ behavior: "smooth" });

    const toolCallMap = new Map<string, string>(); // stream id -> our tool call id

    try {
      const agent = await createPlaygroundAgent(settings);
      const streamResult = await run(agent, text, {
        context: { sessionId: sid },
        stream: true,
      });

      // Type: stream is AsyncIterable and has .completed
      const stream = streamResult as AsyncIterable<{
        type: string;
        item?: {
          type: string;
          id?: string;
          name?: string;
          arguments?: string;
          output?: unknown;
        };
        data?: { type?: string; delta?: string };
        name?: string;
      }> & { completed?: Promise<unknown> };

      await consumeAgentStream(stream, (event) => {
        if (event.type === "text_delta") {
          const msg = messages.find((m) => m.id === assistantMsgId);
          if (msg)
            updateMessage(assistantMsgId, {
              content: msg.content + event.delta,
              streaming: true,
            });
        } else if (event.type === "tool_call") {
          const tcId = `tc-${event.id}-${Date.now()}`;
          toolCallMap.set(event.id, tcId);
          const msg = messages.find((m) => m.id === assistantMsgId);
          const toolCalls = [
            ...(msg?.toolCalls ?? []),
            {
              id: tcId,
              name: event.name,
              args: event.args,
              status: "running" as const,
            },
          ];
          updateMessage(assistantMsgId, { toolCalls, streaming: true });
        } else if (event.type === "tool_result") {
          const tcId = toolCallMap.get(event.id);
          if (tcId)
            updateToolCall(assistantMsgId, tcId, {
              output: event.output,
              status: "done",
            });
        } else if (event.type === "done") {
          updateMessage(assistantMsgId, { streaming: false });
        } else if (event.type === "error") {
          updateMessage(assistantMsgId, {
            content: `Error: ${event.message}`,
            streaming: false,
          });
        }
      });
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      updateMessage(assistantMsgId, {
        content: `Error: ${msg}`,
        streaming: false,
      });
      toast.error("Run failed", { description: msg });
    } finally {
      sending = false;
      await tick();
      messagesEndEl?.scrollIntoView({ behavior: "smooth" });
    }
  }

  onMount(() => {
    settings = getStoredPlaygroundSettings();
  });
</script>

<div class="flex h-[calc(100vh-8rem)] flex-col gap-4">
  <div class="flex shrink-0 items-center justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold tracking-tight">Playground</h1>
      <p class="text-muted-foreground">
        Chat with the coding agent in a sandbox session
      </p>
    </div>
    <div class="flex items-center gap-2">
      <Badge variant="outline" class="text-xs"
        >{PROVIDER_LABELS[settings.provider]} · {settings.model ||
          "default"}</Badge
      >
      {#if sessionId}
        <Badge variant="secondary" class="font-mono text-xs"
          >{sessionId.slice(0, 12)}…</Badge
        >
        <Button
          variant="outline"
          size="sm"
          onclick={endSession}
          disabled={sessionLoading}
        >
          <Square class="mr-1.5 h-4 w-4" />
          End session
        </Button>
      {:else}
        <Button size="sm" onclick={ensureSession} disabled={sessionLoading}>
          {#if sessionLoading}
            <Loader2 class="mr-1.5 h-4 w-4 animate-spin" />
          {:else}
            <Play class="mr-1.5 h-4 w-4" />
          {/if}
          Start session
        </Button>
      {/if}
    </div>
  </div>

  {#if showSettingsPrompt || !hasRequiredConfig(settings)}
    <Card.Root class="shrink-0 border-amber-200 dark:border-amber-800">
      <Card.Header class="py-3">
        <Card.Title class="flex items-center gap-2 text-base">
          <Settings class="h-4 w-4" />
          Playground provider
        </Card.Title>
        <Card.Description
          >Set provider and API key in Settings to use the agent (OpenAI,
          Google, or Google Vertex).</Card.Description
        >
      </Card.Header>
      <Card.Content>
        <a
          href="/settings"
          class="text-sm font-medium text-primary underline hover:no-underline"
          >Open Settings →</a
        >
      </Card.Content>
    </Card.Root>
  {/if}

  <ScrollArea class="min-h-0 flex-1 rounded-lg border bg-muted/30 p-4">
    <div class="space-y-4">
      {#if messages.length === 0}
        <div
          class="flex flex-col items-center justify-center gap-2 py-12 text-center text-muted-foreground"
        >
          <Terminal class="h-12 w-12" />
          <p>
            Start a session, then send a message to run commands in the sandbox.
          </p>
          <p class="text-sm">
            Example: "List files in /workspace" or "Run python3 --version"
          </p>
        </div>
      {:else}
        {#each messages as msg}
          <div
            class="rounded-lg border bg-background p-4 shadow-sm {msg.streaming
              ? 'ring-2 ring-primary/20'
              : ''} {msg.streaming ? 'ring-primary/20' : ''}"
          >
            <div class="mb-2 flex items-center gap-2">
              {#if msg.role === "user"}
                <User class="h-4 w-4 text-blue-600" />
                <span class="font-medium text-blue-600 dark:text-blue-400"
                  >You</span
                >
              {:else}
                <Bot class="h-4 w-4 text-emerald-600" />
                <span class="font-medium text-emerald-600 dark:text-emerald-400"
                  >Assistant</span
                >
                {#if msg.streaming}
                  <Loader2
                    class="h-3.5 w-3.5 animate-spin text-muted-foreground"
                  />
                {/if}
              {/if}
            </div>
            {#if msg.role === "user"}
              <p class="whitespace-pre-wrap text-sm">{msg.content}</p>
            {:else}
              <AssistantMessage
                toolCalls={msg.toolCalls ?? []}
                content={msg.content ?? ""}
                streaming={msg.streaming ?? false}
              />
            {/if}
          </div>
        {/each}
      {/if}
      <div bind:this={messagesEndEl} />
    </div>
  </ScrollArea>

  <form
    class="flex shrink-0 gap-2"
    onsubmit={(e) => {
      e.preventDefault();
      sendMessage();
    }}
  >
    <Input
      bind:value={inputText}
      placeholder="Ask the agent to run commands, write files, etc."
      class="flex-1"
      disabled={sending}
    />
    <Button type="submit" disabled={sending || !inputText.trim()}>
      {#if sending}
        <Loader2 class="mr-2 h-4 w-4 animate-spin" />
      {/if}
      Send
    </Button>
  </form>
</div>
