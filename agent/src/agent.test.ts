import { describe, expect, it } from 'vitest';

import { createMockGqlClient } from '../test/mockGraphql.js';
import { createScriptedModel } from '../test/mockModel.js';
import { runTriage } from './agent.js';
import { loadConfig } from './config.js';

const fleetStatusFixture = {
  units: [{ id: 'u1', callsign: 'M1', status: 'AVAILABLE', capability: 'ALS', zoneId: 'z1', currentCallId: null }],
  calls: [],
  dispatchEvents: [],
};

function makeConfig() {
  return loadConfig();
}

describe('runTriage', () => {
  it('c0: rejects a malformed final answer and proposes nothing', async () => {
    const model = createScriptedModel([{ type: 'text', text: 'not a valid JSON proposal' }]);
    const client = createMockGqlClient({ FleetStatus: { fleetStatus: fleetStatusFixture } });

    const result = await runTriage(makeConfig(), client, { callId: 'call-1', reason: 'ambiguous', attempt: 1 }, model);

    expect(result.stopReason).toBe('INVALID_OUTPUT');
    expect(result.proposal).toBeUndefined();
    expect(client.calls.some((c) => c.document.includes('propose'))).toBe(false);
  });

  it('c0: accepts a well-formed final answer', async () => {
    const proposal = {
      callId: 'call-1',
      actionType: 'NONE',
      targetUnitId: null,
      payload: {},
      reason: 'No action needed after review.',
    };
    const model = createScriptedModel([{ type: 'text', text: JSON.stringify(proposal) }]);
    const client = createMockGqlClient({ FleetStatus: { fleetStatus: fleetStatusFixture } });

    const result = await runTriage(makeConfig(), client, { callId: 'call-1', reason: 'ambiguous', attempt: 1 }, model);

    expect(result.stopReason).toBe('COMPLETED');
    expect(result.proposal).toEqual(proposal);
  });

  it('c4: stops at config.maxSteps when the model only ever calls tools', async () => {
    const config = makeConfig();
    // Script far more tool-call steps than maxSteps allows so the loop is
    // guaranteed to be cut off by the cap, not by running out of script.
    const steps = Array.from({ length: config.maxSteps + 5 }, () => ({
      type: 'tool-call' as const,
      toolName: 'getFleetStatus',
    }));
    const model = createScriptedModel(steps);
    const client = createMockGqlClient({ FleetStatus: { fleetStatus: fleetStatusFixture } });

    const result = await runTriage(config, client, { callId: 'call-1', reason: 'ambiguous', attempt: 1 }, model);

    expect(result.stopReason).toBe('MAX_STEPS');
    expect(result.proposal).toBeUndefined();
  });

  it('c5: writes agent_run and tool_call_log telemetry with token counts, cost, and increasing seq', async () => {
    const proposal = {
      callId: 'call-1',
      actionType: 'NONE',
      targetUnitId: null,
      payload: {},
      reason: 'No action needed.',
    };
    const model = createScriptedModel([
      { type: 'tool-call', toolName: 'getFleetStatus' },
      { type: 'text', text: JSON.stringify(proposal) },
    ]);
    const client = createMockGqlClient({ FleetStatus: { fleetStatus: fleetStatusFixture } });
    const config = makeConfig();

    await runTriage(config, client, { callId: 'call-1', reason: 'ambiguous', attempt: 1 }, model);

    const startCall = client.calls.find((c) => c.document.includes('StartAgentRun'));
    expect(startCall?.variables).toMatchObject({ input: { callId: 'call-1', trigger: 'ambiguous', attempt: 1 } });

    const toolLogCalls = client.calls.filter((c) => c.document.includes('RecordToolCallLog'));
    expect(toolLogCalls.length).toBeGreaterThanOrEqual(1);
    const seqs = toolLogCalls.map((c) => (c.variables?.input as { seq: number }).seq);
    expect(seqs).toEqual([...seqs].sort((a, b) => a - b));
    expect(new Set(seqs).size).toBe(seqs.length);

    const completeCall = client.calls.find((c) => c.document.includes('CompleteAgentRun'));
    const completeInput = completeCall?.variables?.input as {
      status: string;
      promptTokens: number;
      completionTokens: number;
      costUsd: number;
    };
    expect(completeInput.status).toBe('COMPLETED');
    expect(completeInput.promptTokens).toBeGreaterThan(0);
    expect(completeInput.completionTokens).toBeGreaterThan(0);
    expect(completeInput.costUsd).toBeCloseTo(
      (completeInput.promptTokens / 1_000_000) * config.inputPricePerMillionTokens +
        (completeInput.completionTokens / 1_000_000) * config.outputPricePerMillionTokens,
    );
  });
});
