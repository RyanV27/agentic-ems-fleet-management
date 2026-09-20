import type { AddressInfo } from 'node:net';
import http from 'node:http';

import { MastraLanguageModelV2Mock } from '@mastra/core/test-utils/llm-mock';
import { afterEach, describe, expect, it } from 'vitest';

import { createMockGqlClient } from '../test/mockGraphql.js';
import { createServer } from './server.js';
import { loadConfig } from './config.js';

const fleetStatusFixture = { units: [], calls: [], dispatchEvents: [] };

// A doGenerate that never resolves on its own — the test controls exactly
// when each in-flight triage run completes, so concurrency can be observed
// deterministically instead of raced against timers (ROADMAP S7 c7).
function createControllableModel() {
  const releasers: Array<() => void> = [];
  const model = new MastraLanguageModelV2Mock({
    doGenerate: () =>
      new Promise((resolve) => {
        releasers.push(() =>
          resolve({
            content: [
              {
                type: 'text',
                text: JSON.stringify({
                  callId: 'call-x',
                  actionType: 'NONE',
                  targetUnitId: null,
                  payload: {},
                  reason: 'ok',
                }),
              },
            ],
            finishReason: 'stop',
            usage: { inputTokens: 1, outputTokens: 1, totalTokens: 2 },
            warnings: [],
          }),
        );
      }),
  });
  return {
    model,
    get inFlight() {
      return releasers.length;
    },
    release() {
      releasers.shift()?.();
    },
  };
}

async function listen(app: ReturnType<typeof createServer>) {
  const server = http.createServer(app);
  await new Promise<void>((resolve) => server.listen(0, resolve));
  const { port } = server.address() as AddressInfo;
  return { server, port };
}

async function postTriage(port: number, body: unknown) {
  return fetch(`http://localhost:${port}/triage`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(body),
  });
}

// Small helper to yield the event loop so queued async work (agent.generate's
// microtasks, express's request handling) has a chance to run before we
// inspect in-flight state.
function tick() {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('POST /triage', () => {
  let cleanup: (() => void) | undefined;

  afterEach(() => {
    cleanup?.();
    cleanup = undefined;
  });

  it('c7: returns 202 immediately without waiting for the triage run to finish', async () => {
    const { model } = createControllableModel();
    const client = createMockGqlClient({ FleetStatus: { fleetStatus: fleetStatusFixture } });
    const app = createServer(loadConfig(), client, model);
    const { server, port } = await listen(app);
    cleanup = () => server.close();

    const res = await postTriage(port, { callId: 'call-1', reason: 'ambiguous', attempt: 1 });

    expect(res.status).toBe(202);
    expect(await res.json()).toEqual({ accepted: true });
  });

  it('c7: rejects a malformed request body with 400', async () => {
    const client = createMockGqlClient({ FleetStatus: { fleetStatus: fleetStatusFixture } });
    const app = createServer(loadConfig(), client, new MastraLanguageModelV2Mock({ doGenerate: async () => {
      throw new Error('should not be called');
    } }));
    const { server, port } = await listen(app);
    cleanup = () => server.close();

    const res = await postTriage(port, { callId: 'call-1' });

    expect(res.status).toBe(400);
  });

  it('c7: caps concurrent triage runs at config.maxConcurrentTriage and queues the rest instead of rejecting', async () => {
    const config = loadConfig();
    const controllable = createControllableModel();
    const client = createMockGqlClient({ FleetStatus: { fleetStatus: fleetStatusFixture } });
    const app = createServer(config, client, controllable.model);
    const { server, port } = await listen(app);
    cleanup = () => server.close();

    // Send one more request than the concurrency cap allows.
    const requestCount = config.maxConcurrentTriage + 1;
    const responses = await Promise.all(
      Array.from({ length: requestCount }, (_, i) =>
        postTriage(port, { callId: `call-${i}`, reason: 'ambiguous', attempt: 1 }),
      ),
    );

    // Every request is accepted, including the one over the cap — it queues
    // rather than being rejected (ROADMAP S7 c7).
    for (const res of responses) {
      expect(res.status).toBe(202);
    }

    await tick();
    // Only maxConcurrentTriage runs should actually be executing (i.e. have
    // called the model) at once; the excess request is queued in-memory.
    expect(controllable.inFlight).toBe(config.maxConcurrentTriage);

    // Freeing one slot lets the queued request start.
    controllable.release();
    await tick();
    expect(controllable.inFlight).toBe(config.maxConcurrentTriage);

    // Drain everything so the test doesn't leak pending work.
    while (controllable.inFlight > 0) {
      controllable.release();
      await tick();
    }
  });
});
