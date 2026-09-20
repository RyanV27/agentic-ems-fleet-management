import { describe, expect, it } from 'vitest';

import { createMockGqlClient, makeRunContext } from '../../test/mockGraphql.js';
import { createGetIncidentDetailsTool } from './getIncidentDetails.js';

describe('getIncidentDetails', () => {
  const fixture = {
    call: {
      id: 'c1',
      transcript: 'chest pain',
      severity: 1,
      zoneId: 'z1',
      status: 'OPEN',
      assignedUnitId: 'u1',
      destinationHospitalId: null,
    },
    routingDecisions: [{ id: 'r1', path: 'AGENT', matchedRuleId: 'rule-1', decidedAt: '2026-09-20T00:00:00Z' }],
    agentRuns: [{ id: 'run-0', attempt: 1, status: 'COMPLETED', stopReason: 'COMPLETED' }],
    toolCallLogs: [{ id: 't1', seq: 1, toolName: 'getFleetStatus', error: null }],
  };

  it('sends the CallAudit query with the call id and returns the parsed audit', async () => {
    const client = createMockGqlClient({ CallAudit: { callAudit: fixture } });
    const ctx = makeRunContext({}, client);
    const tool = createGetIncidentDetailsTool(ctx);

    const result = await tool.execute!({ callId: 'c1' }, {} as never);

    expect(result).toEqual(fixture);
    expect(client.calls[0]?.document).toContain('CallAudit');
    expect(client.calls[0]?.variables).toEqual({ id: 'c1' });
  });

  it('throws when the call is not found (FR-11)', async () => {
    const client = createMockGqlClient({ CallAudit: { callAudit: null } });
    const ctx = makeRunContext({}, client);
    const tool = createGetIncidentDetailsTool(ctx);

    await expect(tool.execute!({ callId: 'missing' }, {} as never)).rejects.toThrow('no call found');
  });
});
