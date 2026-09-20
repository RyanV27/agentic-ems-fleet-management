import type { GqlClient } from './gql/client.js';
import {
  COMPLETE_AGENT_RUN_MUTATION,
  RECORD_TOOL_CALL_LOG_MUTATION,
  START_AGENT_RUN_MUTATION,
} from './gql/documents.js';

// The agent persists its own run/tool-call audit trail through GraphQL
// instead of writing to a store directly, since its only tool surface is the
// Go GraphQL API (CLAUDE.md conventions, DEC-041, ROADMAP S7 c5).

export type StopReason = 'COMPLETED' | 'MAX_STEPS' | 'INVALID_OUTPUT' | 'ERROR';

interface StartAgentRunResponse {
  startAgentRun: { id: string };
}

export async function startAgentRun(
  client: GqlClient,
  input: { callId: string; trigger: string; attempt: number },
): Promise<string> {
  const data = await client.request<StartAgentRunResponse>(START_AGENT_RUN_MUTATION, { input });
  return data.startAgentRun.id;
}

export async function recordToolCallLog(
  client: GqlClient,
  input: {
    agentRunId: string;
    seq: number;
    toolName: string;
    input: string;
    output: string;
    latencyMs: number;
    error?: string | null;
  },
): Promise<void> {
  await client.request(RECORD_TOOL_CALL_LOG_MUTATION, { input });
}

export async function completeAgentRun(
  client: GqlClient,
  input: {
    agentRunId: string;
    status: 'COMPLETED' | 'FAILED';
    stopReason?: StopReason | null;
    promptTokens: number;
    completionTokens: number;
    costUsd: number;
    latencyMs: number;
  },
): Promise<void> {
  await client.request(COMPLETE_AGENT_RUN_MUTATION, { input });
}

// seqCounter gives each tool call in a run a stable, increasing seq starting
// at 1, matching ToolCallLog.Seq's ordering contract (domain/audit.go).
export function makeSeqCounter(): () => number {
  let seq = 0;
  return () => {
    seq += 1;
    return seq;
  };
}

// RunContext is threaded through every tool factory so each tool call writes
// its own tool_call_logs row without duplicating that logic per tool file
// (ROADMAP S7 c5). callId/attempt are also how write tools derive their
// idempotency key (DEC-040, ROADMAP S7 c6) — they come from the triage
// invocation, not from LLM-supplied tool arguments.
export interface RunContext {
  client: GqlClient;
  callId: string;
  attempt: number;
  agentRunId: string;
  nextSeq: () => number;
}

export function withToolLogging<In, Out>(
  ctx: RunContext,
  toolName: string,
  fn: (input: In) => Promise<Out>,
): (input: In) => Promise<Out> {
  return async (input: In) => {
    const start = Date.now();
    const seq = ctx.nextSeq();
    try {
      const output = await fn(input);
      await recordToolCallLog(ctx.client, {
        agentRunId: ctx.agentRunId,
        seq,
        toolName,
        input: JSON.stringify(input) ?? 'null',
        output: JSON.stringify(output) ?? 'null',
        latencyMs: Date.now() - start,
        error: null,
      });
      return output;
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      await recordToolCallLog(ctx.client, {
        agentRunId: ctx.agentRunId,
        seq,
        toolName,
        input: JSON.stringify(input) ?? 'null',
        output: '',
        latencyMs: Date.now() - start,
        error: message,
      });
      throw err;
    }
  };
}
