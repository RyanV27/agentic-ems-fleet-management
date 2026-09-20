import { describe, expect, it } from 'vitest';

import { createMockGqlClient } from './mockGraphql.js';

// Positive proof that the shared test harness's guard actually fires — every
// tool test relies on this to enforce ROADMAP S7 c3 (write tools never call
// executeAction), so the guard itself needs its own test.
describe('createMockGqlClient', () => {
  it('c3: throws if executeAction is ever sent, regardless of configured responses', async () => {
    const client = createMockGqlClient({});

    await expect(client.request('mutation ExecuteAction($id: ID!) { executeAction(id: $id) { id } }')).rejects.toThrow(
      'executeAction must never be called by the agent',
    );
  });
});
