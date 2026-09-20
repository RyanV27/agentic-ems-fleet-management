import { createTool } from '@mastra/core/tools';
import { z } from 'zod';

import { PROPOSE_REROUTE_UNIT_MUTATION } from '../gql/documents.js';
import { withToolLogging, type RunContext } from '../logging.js';

export const proposeRerouteOutputSchema = z.object({ pendingActionId: z.string() });

interface ProposeRerouteUnitResponse {
  proposeRerouteUnit: { id: string };
}

// Write tool: inserts a PROPOSED action and returns only its id — it never
// mutates fleet state itself and it is never the one that calls
// executeAction (hard rule 1 / DEC-016, ROADMAP S7 c3). The idempotency key
// is derived from the triage run's call id and attempt, not from this tool's
// own arguments, so retriaging the same call/attempt collides on the
// existing action instead of double-queuing (DEC-040, ROADMAP S7 c6).
export function createProposeRerouteTool(ctx: RunContext) {
  return createTool({
    id: 'proposeReroute',
    description: 'Propose rerouting an already-assigned unit onto this call. Does not execute the reroute.',
    inputSchema: z.object({
      unitId: z.string().describe('The unit to reroute onto the call.'),
      rationale: z.string().describe('Why this reroute is proposed.'),
    }),
    outputSchema: proposeRerouteOutputSchema,
    execute: withToolLogging(ctx, 'proposeReroute', async (inputData: { unitId: string; rationale: string }) => {
      const idempotencyKey = `REROUTE_UNIT:${ctx.callId}:${ctx.attempt}`;
      const data = await ctx.client.request<ProposeRerouteUnitResponse>(PROPOSE_REROUTE_UNIT_MUTATION, {
        input: {
          unitId: inputData.unitId,
          callId: ctx.callId,
          rationale: inputData.rationale,
          proposedBy: 'AGENT',
          agentRunId: ctx.agentRunId,
          idempotencyKey,
        },
      });
      return { pendingActionId: data.proposeRerouteUnit.id };
    }),
  });
}
