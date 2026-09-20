import { describe, expect, it } from 'vitest';

import { createMockGqlClient, makeRunContext } from '../../test/mockGraphql.js';
import { createGetFleetStatusTool } from './getFleetStatus.js';

describe('getFleetStatus', () => {
  it('sends the FleetStatus query and returns the parsed fleet snapshot', async () => {
    const fixture = {
      units: [{ id: 'u1', callsign: 'M1', status: 'AVAILABLE', capability: 'ALS', zoneId: 'z1', currentCallId: null }],
      calls: [{ id: 'c1', status: 'OPEN', severity: 1, zoneId: 'z1', assignedUnitId: null, priorityScore: 1000 }],
      dispatchEvents: [{ id: 'd1', type: 'DELAY', callId: 'c1', unitId: 'u1', status: 'OPEN', note: 'traffic' }],
    };
    const client = createMockGqlClient({ FleetStatus: { fleetStatus: fixture } });
    const ctx = makeRunContext({}, client);
    const tool = createGetFleetStatusTool(ctx);

    const result = await tool.execute!({}, {} as never);

    expect(result).toEqual(fixture);
    expect(client.calls[0]?.document).toContain('FleetStatus');
  });

  it('propagates errors from the GraphQL client (FR-11)', async () => {
    const client = createMockGqlClient({
      FleetStatus: () => {
        throw new Error('boom');
      },
    });
    const ctx = makeRunContext({}, client);
    const tool = createGetFleetStatusTool(ctx);

    await expect(tool.execute!({}, {} as never)).rejects.toThrow('boom');
  });
});
