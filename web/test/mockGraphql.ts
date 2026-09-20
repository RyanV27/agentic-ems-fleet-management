import type { GqlClient } from '../src/gql/client';

// Mirrors agent/test/mockGraphql.ts's shape: matches a mocked response by
// checking whether the sent document string contains an operation-name key.
export interface MockGqlClient extends GqlClient {
  calls: Array<{ document: string; variables?: Record<string, unknown> }>;
}

type MockResponse = unknown | ((variables: Record<string, unknown>) => unknown);

export function createMockGqlClient(responses: Record<string, MockResponse>): MockGqlClient {
  const calls: MockGqlClient['calls'] = [];
  return {
    calls,
    async request<T>(document: string, variables?: Record<string, unknown>): Promise<T> {
      calls.push({ document, variables });
      const key = Object.keys(responses).find((k) => document.includes(k));
      if (!key) {
        throw new Error(
          `mockGqlClient: no response configured for document containing any of [${Object.keys(responses).join(', ')}]`,
        );
      }
      const value = responses[key];
      const resolved = typeof value === 'function' ? (value as (v: Record<string, unknown>) => T)(variables ?? {}) : value;
      return resolved as T;
    },
  };
}
