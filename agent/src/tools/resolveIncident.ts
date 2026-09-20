import { createTool } from '@mastra/core/tools';
import { z } from 'zod';

import { PROPOSE_RESOLVE_EVENT_MUTATION } from '../gql/documents.js';
import { withToolLogging, type RunContext } from '../logging.js';

export const resolveIncidentOutputSchema = z.object({ pendingActionId: z.string() });

interface ProposeResolveEventResponse {
  proposeResolveEvent: { id: string };
}

// Write tool: inserts a PROPOSED action and returns only its id (hard rule 1
// / DEC-016, ROADMAP S7 c3). See proposeReroute.ts for the idempotency-key
// derivation rationale (DEC-040, ROADMAP S7 c6).
export function createResolveIncidentTool(ctx: RunContext) {
  return createTool({
    id: 'resolveIncident',
    description:
      'Propose resolving an open dispatch event for this call (e.g. a delay or breakdown is no longer relevant). Does not execute the resolution.',
    inputSchema: z.object({
      dispatchEventId: z.string().describe('The open dispatch event to resolve.'),
      rationale: z.string().describe('Why this dispatch event should be resolved.'),
    }),
    outputSchema: resolveIncidentOutputSchema,
    execute: withToolLogging(ctx, 'resolveIncident', async (inputData: { dispatchEventId: string; rationale: string }) => {
      const idempotencyKey = `RESOLVE_EVENT:${ctx.callId}:${ctx.attempt}`;
      const data = await ctx.client.request<ProposeResolveEventResponse>(PROPOSE_RESOLVE_EVENT_MUTATION, {
        input: {
          dispatchEventId: inputData.dispatchEventId,
          rationale: inputData.rationale,
          proposedBy: 'AGENT',
          agentRunId: ctx.agentRunId,
          idempotencyKey,
        },
      });
      return { pendingActionId: data.proposeResolveEvent.id };
    }),
  });
}
