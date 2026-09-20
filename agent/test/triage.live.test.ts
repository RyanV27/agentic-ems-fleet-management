import { execFileSync, spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { afterAll, beforeAll, describe, expect, it } from 'vitest';

import { runTriage } from '../src/agent.js';
import { loadConfig } from '../src/config.js';
import { createGqlClient, type GqlClient } from '../src/gql/client.js';

// ROADMAP S7 c9: "A live test (`npm run test:live -w agent`) runs one real
// Sonnet 5 triage against a seeded service and asserts a PROPOSED action was
// created and fleet state is unchanged." This is the one test in the repo
// that spends real money (Anthropic + OpenRouter) and spawns the real Go
// fleet server as a child process — everything else in agent/test is a
// mocked-client unit test (ROADMAP S7 c2).
//
// The Go server is started with AGENT_ENABLED=false so submitTranscript only
// does the synchronous half of intake (extraction, routing decision, marking
// the call ESCALATED) — it never tries to reach a TS /triage endpoint itself
// (services/fleet/internal/intake/intake.go). This test then drives the
// agent side directly via runTriage, exactly like server.ts's /triage
// handler would, but without needing a second process.
const hasLiveKeys = Boolean(process.env.ANTHROPIC_API_KEY) && Boolean(process.env.OPENROUTER_API_KEY);

const FLEET_PORT = 18099;
const FLEET_URL = `http://localhost:${FLEET_PORT}/query`;
const REPO_ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..');

// Deliberately garbled/self-contradicting so extraction confidence lands
// below EXTRACTION_CONFIDENCE_THRESHOLD (0.75) and the deterministic rules
// engine escalates via RuleLowConfidence — the only escalation reason
// reachable without first manufacturing fleet contention state
// (services/fleet/internal/rules/rules.go).
const AMBIGUOUS_TRANSCRIPT =
  '...static... caller reports maybe chest pain or possibly a fall, unit- unclear, ' +
  'location could be either zone-1 or zone-2, caller keeps contradicting themselves ' +
  'about whether the patient is conscious... signal breaking up... unable to confirm severity...';

let fleetProcess: ChildProcess | undefined;
let client: GqlClient;

async function waitForFleetReady(timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  let lastErr: unknown;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(FLEET_URL, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: '{ zones { id } }' }),
      });
      if (res.ok) return;
    } catch (err) {
      lastErr = err;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(`fleet server did not become ready within ${timeoutMs}ms: ${String(lastErr)}`);
}

describe.skipIf(!hasLiveKeys)('S7 c9: live triage against a real fleet service', () => {
  beforeAll(async () => {
    const workDir = mkdtempSync(join(tmpdir(), 'fleet-live-'));
    const dbPath = join(workDir, 'fleet.db');
    // Spawn the compiled binary directly rather than `go run`: `go run`
    // builds and then execs the real server as a *separate child process*
    // (on Windows in particular, killing the `go run` wrapper does not kill
    // that grandchild), which previously leaked a server.exe still listening
    // on FLEET_PORT and serving an old temp DB to every later test run.
    const binaryPath = join(workDir, process.platform === 'win32' ? 'fleet-server.exe' : 'fleet-server');
    execFileSync('go', ['build', '-o', binaryPath, './services/fleet/cmd/server'], { cwd: REPO_ROOT });

    fleetProcess = spawn(binaryPath, [], {
      env: {
        ...process.env,
        FLEET_DB_PATH: dbPath,
        FLEET_PORT: String(FLEET_PORT),
        AGENT_ENABLED: 'false',
      },
      stdio: 'pipe',
    });

    client = createGqlClient(FLEET_URL);
    await waitForFleetReady(30_000);
  }, 60_000);

  afterAll(() => {
    fleetProcess?.kill();
  });

  it(
    'triages an escalated call and proposes an action without changing fleet state',
    async () => {
      const config = loadConfig();

      const submitted = await client.request<{ submitTranscript: { id: string; status: string } }>(
        `mutation Submit($input: SubmitTranscriptInput!) {
          submitTranscript(input: $input) { id status }
        }`,
        // zoneHint (OQ-13) is a fallback used only when extraction fails to
        // produce a zone at all (services/fleet/internal/intake/intake.go);
        // AMBIGUOUS_TRANSCRIPT is designed to make extraction fail/low-confidence,
        // so without it the call would persist with an empty zone_id and fail
        // the DB's foreign key constraint.
        { input: { transcript: AMBIGUOUS_TRANSCRIPT, zoneHint: 'zone-1' } },
      );
      expect(submitted.submitTranscript.status).toBe('ESCALATED');
      const callId = submitted.submitTranscript.id;

      const before = await client.request<{ fleetStatus: { units: unknown[] } }>(
        '{ fleetStatus { units { id status capability zoneId currentCallId statusChangedAt } } }',
      );

      const result = await runTriage(config, client, {
        callId,
        reason: 'RuleLowConfidence',
        attempt: 1,
      });

      expect(result.stopReason).toBe('COMPLETED');
      expect(result.proposal).toBeDefined();

      const after = await client.request<{ fleetStatus: { units: unknown[] } }>(
        '{ fleetStatus { units { id status capability zoneId currentCallId statusChangedAt } } }',
      );
      expect(after.fleetStatus.units).toEqual(before.fleetStatus.units);

      const pending = await client.request<{
        pendingActions: Array<{ id: string; status: string; proposedBy: string; agentRunId: string | null }>;
      }>('query($status: ActionStatus) { pendingActions(status: $status) { id status proposedBy agentRunId } }', {
        status: 'PROPOSED',
      });
      // Real-model runs are occasionally non-deterministic about actually
      // invoking the write tool before answering; log enough to diagnose a
      // flaky failure without re-running (this test costs real money).
      const proposedByThisRun = pending.pendingActions.find((a) => a.agentRunId === result.agentRunId);
      if (!proposedByThisRun) {
        console.error('[debug] agentRunId:', result.agentRunId);
        console.error('[debug] proposal:', JSON.stringify(result.proposal));
        console.error('[debug] pendingActions:', JSON.stringify(pending.pendingActions));
      }
      expect(proposedByThisRun).toBeDefined();
      expect(proposedByThisRun?.proposedBy).toBe('AGENT');
    },
    120_000,
  );
});
