import { describe, expect, it } from 'vitest';

import { createMockGqlClient, makeRunContext } from '../../test/mockGraphql.js';
import { createProposeRerouteTool } from './proposeReroute.js';

describe('proposeReroute', () => {
  it('sends a proposeRerouteUnit mutation with a derived idempotency key and returns only the pending action id', async () => {
    const client = createMockGqlClient({ ProposeRerouteUnit: { proposeRerouteUnit: { id: 'pa-1' } } });
    const ctx = makeRunContext({ callId: 'call-9', attempt: 2, agentRunId: 'run-9' }, client);
    const tool = createProposeRerouteTool(ctx);

    const result = await tool.execute!({ unitId: 'u2', rationale: 'closer unit available' }, {} as never);

    expect(result).toEqual({ pendingActionId: 'pa-1' });
    expect(client.calls[0]?.document).toContain('proposeRerouteUnit');
    expect(client.calls[0]?.document).toMatch(/^\s*mutation/);
    expect(client.calls[0]?.variables).toEqual({
      input: {
        unitId: 'u2',
        callId: 'call-9',
        rationale: 'closer unit available',
        proposedBy: 'AGENT',
        agentRunId: 'run-9',
        idempotencyKey: 'REROUTE_UNIT:call-9:2',
      },
    });
  });

  it('propagates errors from the GraphQL client (FR-11)', async () => {
    const client = createMockGqlClient({
      ProposeRerouteUnit: () => {
        throw new Error('rejected');
      },
    });
    const ctx = makeRunContext({}, client);
    const tool = createProposeRerouteTool(ctx);

    await expect(tool.execute!({ unitId: 'u2', rationale: 'x' }, {} as never)).rejects.toThrow('rejected');
  });
});
