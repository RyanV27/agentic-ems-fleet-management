import { createTool } from '@mastra/core/tools';
import { z } from 'zod';

import { FLEET_STATUS_QUERY } from '../gql/documents.js';
import { withToolLogging, type RunContext } from '../logging.js';

const unitSchema = z.object({
  id: z.string(),
  callsign: z.string(),
  status: z.string(),
  capability: z.string(),
  zoneId: z.string(),
  currentCallId: z.string().nullable(),
});

const callSchema = z.object({
  id: z.string(),
  status: z.string(),
  severity: z.number(),
  zoneId: z.string(),
  assignedUnitId: z.string().nullable(),
  priorityScore: z.number().nullable(),
});

const dispatchEventSchema = z.object({
  id: z.string(),
  type: z.string(),
  callId: z.string().nullable(),
  unitId: z.string().nullable(),
  status: z.string(),
  note: z.string(),
});

export const getFleetStatusOutputSchema = z.object({
  units: z.array(unitSchema),
  calls: z.array(callSchema),
  dispatchEvents: z.array(dispatchEventSchema),
});

interface FleetStatusResponse {
  fleetStatus: z.infer<typeof getFleetStatusOutputSchema>;
}

// Read tool: executes immediately, no approval gate (ARCHITECTURE.md §6
// boundary 3, ROADMAP S7 c1).
export function createGetFleetStatusTool(ctx: RunContext) {
  return createTool({
    id: 'getFleetStatus',
    description: 'Get the current status of every unit, call, and open dispatch event in the fleet.',
    inputSchema: z.object({}),
    outputSchema: getFleetStatusOutputSchema,
    execute: withToolLogging(ctx, 'getFleetStatus', async () => {
      const data = await ctx.client.request<FleetStatusResponse>(FLEET_STATUS_QUERY);
      return data.fleetStatus;
    }),
  });
}
