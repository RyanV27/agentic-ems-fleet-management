import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { Agent } from '@mastra/core/agent';
import type { MastraModelConfig } from '@mastra/core/llm';
import { z } from 'zod';

import { computeCostUsd, type Config } from './config.js';
import type { GqlClient } from './gql/client.js';
import { completeAgentRun, makeSeqCounter, startAgentRun, type RunContext, type StopReason } from './logging.js';
import { createAssignBackupUnitTool } from './tools/assignBackupUnit.js';
import { createGetFleetStatusTool } from './tools/getFleetStatus.js';
import { createGetIncidentDetailsTool } from './tools/getIncidentDetails.js';
import { createProposeRerouteTool } from './tools/proposeReroute.js';
import { createResolveIncidentTool } from './tools/resolveIncident.js';

// System prompt lives in a versioned file, not an inline string literal, so
// evals can target it (CLAUDE.md conventions, ROADMAP S7 c8).
const PROMPT_PATH = join(dirname(fileURLToPath(import.meta.url)), '..', 'prompts', 'triage.md');
let cachedInstructions: string | undefined;
function loadInstructions(): string {
  cachedInstructions ??= readFileSync(PROMPT_PATH, 'utf-8');
  return cachedInstructions;
}

export const triageProposalSchema = z.object({
  callId: z.string(),
  actionType: z.enum(['REROUTE_UNIT', 'ASSIGN_BACKUP_UNIT', 'RESOLVE_EVENT', 'NONE']),
  targetUnitId: z.string().nullable(),
  payload: z.record(z.string(), z.unknown()),
  reason: z.string().max(240),
});
export type TriageProposal = z.infer<typeof triageProposalSchema>;

export interface TriageRequest {
  callId: string;
  reason: string;
  attempt: number;
}

export interface TriageResult {
  agentRunId: string;
  stopReason: StopReason;
  proposal?: TriageProposal;
}

function buildTools(ctx: RunContext) {
  return {
    getFleetStatus: createGetFleetStatusTool(ctx),
    getIncidentDetails: createGetIncidentDetailsTool(ctx),
    proposeReroute: createProposeRerouteTool(ctx),
    assignBackupUnit: createAssignBackupUnitTool(ctx),
    resolveIncident: createResolveIncidentTool(ctx),
  };
}

// Models are told not to wrap the response in markdown, but Sonnet 5
// sometimes fences it anyway (observed in ROADMAP S7 c9's live test) — strip
// a leading/trailing ```json fence before parsing.
const FENCE_PATTERN = /^```(?:json)?\s*([\s\S]*?)\s*```$/;

function tryParseJson(text: string): unknown {
  const fenceMatch = FENCE_PATTERN.exec(text.trim());
  const candidate = fenceMatch?.[1] ?? text;
  try {
    return JSON.parse(candidate);
  } catch {
    return undefined;
  }
}

// runTriage is the one entry point boundary 4 (ARCHITECTURE.md §6) describes:
// it creates the AgentRun row, runs a bounded tool-calling loop against the
// real Anthropic model (or an injected test model), validates the final
// output against triageProposalSchema, and always finalizes the AgentRun —
// no proposal is ever read from an unvalidated or erroring run (ROADMAP S7
// c0, c4, c5).
export async function runTriage(
  config: Config,
  client: GqlClient,
  req: TriageRequest,
  // Injectable for tests (MastraLanguageModelV2Mock) — production callers
  // omit this and get config.agentModel (ROADMAP S7 c9's live test uses the
  // real Sonnet 5 model this way).
  model?: MastraModelConfig,
): Promise<TriageResult> {
  const startedAt = Date.now();
  const agentRunId = await startAgentRun(client, {
    callId: req.callId,
    trigger: req.reason,
    attempt: req.attempt,
  });

  const ctx: RunContext = {
    client,
    callId: req.callId,
    attempt: req.attempt,
    agentRunId,
    nextSeq: makeSeqCounter(),
  };

  const agent = new Agent({
    id: 'triage-agent',
    name: 'Triage Agent',
    instructions: loadInstructions(),
    model: model ?? config.agentModel,
    tools: buildTools(ctx),
  });

  let promptTokens = 0;
  let completionTokens = 0;
  let stopReason: StopReason;
  let proposal: TriageProposal | undefined;

  try {
    const result = await agent.generate(
      `Call ${req.callId} was escalated. Reason: ${req.reason}. Investigate and respond with the required JSON object.`,
      { maxSteps: config.maxSteps },
    );
    promptTokens = result.usage?.inputTokens ?? 0;
    completionTokens = result.usage?.outputTokens ?? 0;

    const hitStepCap = result.steps.length >= config.maxSteps && result.finishReason !== 'stop';
    if (hitStepCap) {
      stopReason = 'MAX_STEPS';
    } else {
      const parsed = triageProposalSchema.safeParse(tryParseJson(result.text));
      if (parsed.success) {
        stopReason = 'COMPLETED';
        proposal = parsed.data;
      } else {
        stopReason = 'INVALID_OUTPUT';
      }
    }
  } catch {
    stopReason = 'ERROR';
  }

  await completeAgentRun(client, {
    agentRunId,
    status: stopReason === 'ERROR' ? 'FAILED' : 'COMPLETED',
    stopReason,
    promptTokens,
    completionTokens,
    costUsd: computeCostUsd(config, promptTokens, completionTokens),
    latencyMs: Date.now() - startedAt,
  });

  return { agentRunId, stopReason, proposal };
}
