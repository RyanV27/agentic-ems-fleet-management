import { describe, expect, it } from 'vitest';

import { createMockGqlClient, makeRunContext } from '../../test/mockGraphql.js';
import { createResolveIncidentTool } from './resolveIncident.js';

describe('resolveIncident', () => {
  it('sends a proposeResolveEvent mutation with a derived idempotency key and returns only the pending action id', async () => {
    const client = createMockGqlClient({ ProposeResolveEvent: { proposeResolveEvent: { id: 'pa-3' } } });
    const ctx = makeRunContext({ callId: 'call-7', attempt: 3, agentRunId: 'run-7' }, client);
    const tool = createResolveIncidentTool(ctx);

    const result = await tool.execute!({ dispatchEventId: 'd1', rationale: 'delay no longer relevant' }, {} as never);

    expect(result).toEqual({ pendingActionId: 'pa-3' });
    expect(client.calls[0]?.document).toContain('proposeResolveEvent');
    expect(client.calls[0]?.variables).toEqual({
      input: {
        dispatchEventId: 'd1',
        rationale: 'delay no longer relevant',
        proposedBy: 'AGENT',
        agentRunId: 'run-7',
        idempotencyKey: 'RESOLVE_EVENT:call-7:3',
      },
    });
  });

  it('propagates errors from the GraphQL client (FR-11)', async () => {
    const client = createMockGqlClient({
      ProposeResolveEvent: () => {
        throw new Error('rejected');
      },
    });
    const ctx = makeRunContext({}, client);
    const tool = createResolveIncidentTool(ctx);

    await expect(tool.execute!({ dispatchEventId: 'd1', rationale: 'x' }, {} as never)).rejects.toThrow('rejected');
  });
});
