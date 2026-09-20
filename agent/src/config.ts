import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

// Falls back to the repo-root .env file for any variable not already set in
// the real process environment — mirrors services/fleet/internal/config's
// envLookup (real env vars always win over .env). Needed because
// ANTHROPIC_API_KEY / OPENROUTER_API_KEY are read directly from
// process.env by the Anthropic/OpenRouter SDKs, not through Config, so the
// fallback has to land in process.env itself rather than just this module's
// return value.
const REPO_ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..');

function loadDotEnvFallback(): void {
  let data: string;
  try {
    data = readFileSync(join(REPO_ROOT, '.env'), 'utf-8');
  } catch {
    return;
  }
  for (const line of data.split('\n')) {
    const trimmed = line.trim();
    if (trimmed === '' || trimmed.startsWith('#')) continue;
    const eq = trimmed.indexOf('=');
    if (eq === -1) continue;
    const key = trimmed.slice(0, eq).trim();
    if (process.env[key] === undefined) {
      process.env[key] = trimmed.slice(eq + 1).trim();
    }
  }
}

loadDotEnvFallback();

// Config constants are env-driven (CLAUDE.md conventions) — never hardcode
// AGENT_MAX_STEPS / AGENT_MAX_CONCURRENT_TRIAGE / etc. in code or tests.
export interface Config {
  fleetGraphqlUrl: string;
  agentModel: string;
  maxSteps: number;
  maxConcurrentTriage: number;
  port: number;
  // Anthropic list pricing per million tokens (OQ-15, NON-BLOCKING) — cost is
  // a nice-to-have audit field (ROADMAP S7 c5), not a gating criterion.
  inputPricePerMillionTokens: number;
  outputPricePerMillionTokens: number;
}

function envInt(name: string, fallback: number): number {
  const raw = process.env[name];
  if (raw === undefined || raw === '') return fallback;
  const parsed = Number.parseInt(raw, 10);
  if (Number.isNaN(parsed)) throw new Error(`${name} must be an integer, got ${raw}`);
  return parsed;
}

function envFloat(name: string, fallback: number): number {
  const raw = process.env[name];
  if (raw === undefined || raw === '') return fallback;
  const parsed = Number.parseFloat(raw);
  if (Number.isNaN(parsed)) throw new Error(`${name} must be a number, got ${raw}`);
  return parsed;
}

export function loadConfig(): Config {
  return {
    fleetGraphqlUrl: process.env.FLEET_GRAPHQL_URL ?? 'http://localhost:8080/query',
    agentModel: process.env.AGENT_MODEL ?? 'anthropic/claude-sonnet-5',
    maxSteps: envInt('AGENT_MAX_STEPS', 8),
    maxConcurrentTriage: envInt('AGENT_MAX_CONCURRENT_TRIAGE', 2),
    port: envInt('AGENT_PORT', 8081),
    inputPricePerMillionTokens: envFloat('AGENT_INPUT_PRICE_PER_MTOK', 3),
    outputPricePerMillionTokens: envFloat('AGENT_OUTPUT_PRICE_PER_MTOK', 15),
  };
}

export function computeCostUsd(config: Config, promptTokens: number, completionTokens: number): number {
  return (
    (promptTokens / 1_000_000) * config.inputPricePerMillionTokens +
    (completionTokens / 1_000_000) * config.outputPricePerMillionTokens
  );
}
