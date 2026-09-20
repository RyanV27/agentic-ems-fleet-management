import type { MastraModelConfig } from '@mastra/core/llm';
import express from 'express';

import { runTriage, type TriageRequest } from './agent.js';
import { loadConfig, type Config } from './config.js';
import { createGqlClient, type GqlClient } from './gql/client.js';

// POST /triage returns 202 immediately and runs the agent asynchronously
// (DEC-008: escalation is fire-and-forget). At most config.maxConcurrentTriage
// runs execute at once (DEC-018); the rest queue in-memory and are drained as
// slots free up — a request is never rejected for being over the cap
// (ROADMAP S7 c7).
export function createServer(
  config: Config = loadConfig(),
  client: GqlClient = createGqlClient(config.fleetGraphqlUrl),
  // Injectable for tests (MastraLanguageModelV2Mock), mirroring runTriage's
  // own test-injection pattern — production callers omit this.
  model?: MastraModelConfig,
) {
  const app = express();
  app.use(express.json());

  let active = 0;
  const queue: TriageRequest[] = [];

  function pump(): void {
    while (active < config.maxConcurrentTriage && queue.length > 0) {
      const next = queue.shift();
      if (!next) break;
      active += 1;
      runTriage(config, client, next, model)
        .catch((err: unknown) => {
          console.error(`triage run failed for call ${next.callId}:`, err);
        })
        .finally(() => {
          active -= 1;
          pump();
        });
    }
  }

  app.post('/triage', (req, res) => {
    const body = req.body as Partial<TriageRequest>;
    if (typeof body.callId !== 'string' || typeof body.reason !== 'string' || typeof body.attempt !== 'number') {
      res.status(400).json({ error: 'callId (string), reason (string), and attempt (number) are required' });
      return;
    }
    queue.push({ callId: body.callId, reason: body.reason, attempt: body.attempt });
    res.status(202).json({ accepted: true });
    pump();
  });

  return app;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const config = loadConfig();
  createServer(config).listen(config.port, () => {
    console.log(`agent listening on :${config.port}`);
  });
}
