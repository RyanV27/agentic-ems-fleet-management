import type { GqlClient } from '../src/gql/client.js';

// Shared test harness for tools/agent code that depends on GqlClient.
// Matches a mocked response by checking whether the sent document string
// contains an operation-name key (e.g. 'FleetStatus', 'ProposeRerouteUnit').
// Guards against ever sending executeAction — the agent must never call it
// (hard rule 1 / DEC-016, ROADMAP S7 c3).
export interface MockGqlClient extends GqlClient {
  calls: Array<{ document: string; variables?: Record<string, unknown> }>;
}

type MockResponse = unknown | ((variables: Record<string, unknown>) => unknown);

// Every tool call also triggers a recordToolCallLog write (ROADMAP S7 c5,
// via withToolLogging). Tests that only care about the tool's own request
// shouldn't have to stub this bookkeeping call every time, so it defaults to
// a no-op success unless a test overrides it.
const DEFAULT_RESPONSES: Record<string, MockResponse> = {
  RecordToolCallLog: { recordToolCallLog: { id: 'mock-log-id' } },
  StartAgentRun: { startAgentRun: { id: 'mock-run-id' } },
  CompleteAgentRun: { completeAgentRun: { id: 'mock-run-id' } },
};

export function createMockGqlClient(responses: Record<string, MockResponse>): MockGqlClient {
  const calls: MockGqlClient['calls'] = [];
  const merged = { ...DEFAULT_RESPONSES, ...responses };
  return {
    calls,
    async request<T>(document: string, variables?: Record<string, unknown>): Promise<T> {
      calls.push({ document, variables });
      if (document.includes('executeAction')) {
        throw new Error('mockGqlClient: executeAction must never be called by the agent (ROADMAP S7 c3)');
      }
      const key = Object.keys(merged).find((k) => document.includes(k));
      if (!key) {
        throw new Error(
          `mockGqlClient: no response configured for document containing any of [${Object.keys(merged).join(', ')}]`,
        );
      }
      const value = merged[key];
      const resolved = typeof value === 'function' ? (value as (v: Record<string, unknown>) => T)(variables ?? {}) : value;
      return resolved as T;
    },
  };
}

export function makeRunContext(overrides: Partial<{ callId: string; attempt: number; agentRunId: string }> = {}, client?: GqlClient) {
  return {
    client: client ?? createMockGqlClient({}),
    callId: overrides.callId ?? 'call-1',
    attempt: overrides.attempt ?? 1,
    agentRunId: overrides.agentRunId ?? 'run-1',
    nextSeq: (() => {
      let seq = 0;
      return () => {
        seq += 1;
        return seq;
      };
    })(),
  };
}
