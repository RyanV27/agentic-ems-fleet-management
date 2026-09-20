import { describe, expect, it } from 'vitest';

import { createMockGqlClient, makeRunContext } from '../../test/mockGraphql.js';
import { createAssignBackupUnitTool } from './assignBackupUnit.js';

describe('assignBackupUnit', () => {
  it('sends a proposeAssignBackupUnit mutation with a derived idempotency key and returns only the pending action id', async () => {
    const client = createMockGqlClient({ ProposeAssignBackupUnit: { proposeAssignBackupUnit: { id: 'pa-2' } } });
    const ctx = makeRunContext({ callId: 'call-5', attempt: 1, agentRunId: 'run-5' }, client);
    const tool = createAssignBackupUnitTool(ctx);

    const result = await tool.execute!({ unitId: 'u3', rationale: 'high severity, needs backup' }, {} as never);

    expect(result).toEqual({ pendingActionId: 'pa-2' });
    expect(client.calls[0]?.document).toContain('proposeAssignBackupUnit');
    expect(client.calls[0]?.variables).toEqual({
      input: {
        callId: 'call-5',
        unitId: 'u3',
        rationale: 'high severity, needs backup',
        proposedBy: 'AGENT',
        agentRunId: 'run-5',
        idempotencyKey: 'ASSIGN_BACKUP_UNIT:call-5:1',
      },
    });
  });

  it('propagates errors from the GraphQL client (FR-11)', async () => {
    const client = createMockGqlClient({
      ProposeAssignBackupUnit: () => {
        throw new Error('rejected');
      },
    });
    const ctx = makeRunContext({}, client);
    const tool = createAssignBackupUnitTool(ctx);

    await expect(tool.execute!({ unitId: 'u3', rationale: 'x' }, {} as never)).rejects.toThrow('rejected');
  });
});
