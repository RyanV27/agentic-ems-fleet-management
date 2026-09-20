// Vite exposes only VITE_-prefixed vars to client code, read via
// import.meta.env — the browser-bundle equivalent of agent/src/config.ts's
// Node process.env pattern, but a different mechanism (no .env fallback
// shim needed here; Vite reads the repo-root .env itself via envDir).
export interface Config {
  fleetGraphqlUrl: string;
  agentBaseUrl: string;
  pollIntervalMs: number;
}

function envInt(value: string | undefined, fallback: number): number {
  if (value === undefined || value === '') return fallback;
  const parsed = Number.parseInt(value, 10);
  if (Number.isNaN(parsed)) throw new Error(`invalid integer env value: ${value}`);
  return parsed;
}

export function loadConfig(): Config {
  const env = import.meta.env;
  return {
    fleetGraphqlUrl: env.VITE_FLEET_GRAPHQL_URL ?? 'http://localhost:8080/query',
    agentBaseUrl: env.VITE_AGENT_BASE_URL ?? 'http://localhost:8081',
    pollIntervalMs: envInt(env.VITE_POLL_INTERVAL_MS, 5000),
  };
}
