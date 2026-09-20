import { createTool } from '@mastra/core/tools';
import { z } from 'zod';

import { CALL_AUDIT_QUERY } from '../gql/documents.js';
import { withToolLogging, type RunContext } from '../logging.js';

export const getIncidentDetailsOutputSchema = z.object({
  call: z.object({
    id: z.string(),
    transcript: z.string(),
    severity: z.number(),
    zoneId: z.string(),
    status: z.string(),
    assignedUnitId: z.string().nullable(),
    destinationHospitalId: z.string().nullable(),
  }),
  routingDecisions: z.array(
    z.object({
      id: z.string(),
      path: z.string(),
      matchedRuleId: z.string(),
      decidedAt: z.string(),
    }),
  ),
  agentRuns: z.array(
    z.object({
      id: z.string(),
      attempt: z.number(),
      status: z.string(),
      stopReason: z.string().nullable(),
    }),
  ),
  toolCallLogs: z.array(
    z.object({
      id: z.string(),
      seq: z.number(),
      toolName: z.string(),
      error: z.string().nullable(),
    }),
  ),
});

interface CallAuditResponse {
  callAudit: z.infer<typeof getIncidentDetailsOutputSchema> | null;
}

// Read tool: executes immediately, no approval gate (ARCHITECTURE.md §6
// boundary 3, ROADMAP S7 c1).
export function createGetIncidentDetailsTool(ctx: RunContext) {
  return createTool({
    id: 'getIncidentDetails',
    description: 'Get the transcript, routing history, and prior agent runs for one call by id.',
    inputSchema: z.object({ callId: z.string() }),
    outputSchema: getIncidentDetailsOutputSchema,
    execute: withToolLogging(ctx, 'getIncidentDetails', async (inputData: { callId: string }) => {
      const data = await ctx.client.request<CallAuditResponse>(CALL_AUDIT_QUERY, { id: inputData.callId });
      if (!data.callAudit) {
        throw new Error(`getIncidentDetails: no call found for id ${inputData.callId}`);
      }
      return data.callAudit;
    }),
  });
}
