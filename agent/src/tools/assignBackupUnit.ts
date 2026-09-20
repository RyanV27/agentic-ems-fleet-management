import { createTool } from '@mastra/core/tools';
import { z } from 'zod';

import { PROPOSE_ASSIGN_BACKUP_UNIT_MUTATION } from '../gql/documents.js';
import { withToolLogging, type RunContext } from '../logging.js';

export const assignBackupUnitOutputSchema = z.object({ pendingActionId: z.string() });

interface ProposeAssignBackupUnitResponse {
  proposeAssignBackupUnit: { id: string };
}

// Write tool: inserts a PROPOSED action and returns only its id (hard rule 1
// / DEC-016, ROADMAP S7 c3). See proposeReroute.ts for the idempotency-key
// derivation rationale (DEC-040, ROADMAP S7 c6).
export function createAssignBackupUnitTool(ctx: RunContext) {
  return createTool({
    id: 'assignBackupUnit',
    description: 'Propose assigning a second, backup unit to this call. Does not execute the assignment.',
    inputSchema: z.object({
      unitId: z.string().describe('The backup unit to assign to the call.'),
      rationale: z.string().describe('Why a backup unit is proposed.'),
    }),
    outputSchema: assignBackupUnitOutputSchema,
    execute: withToolLogging(ctx, 'assignBackupUnit', async (inputData: { unitId: string; rationale: string }) => {
      const idempotencyKey = `ASSIGN_BACKUP_UNIT:${ctx.callId}:${ctx.attempt}`;
      const data = await ctx.client.request<ProposeAssignBackupUnitResponse>(PROPOSE_ASSIGN_BACKUP_UNIT_MUTATION, {
        input: {
          callId: ctx.callId,
          unitId: inputData.unitId,
          rationale: inputData.rationale,
          proposedBy: 'AGENT',
          agentRunId: ctx.agentRunId,
          idempotencyKey,
        },
      });
      return { pendingActionId: data.proposeAssignBackupUnit.id };
    }),
  });
}
