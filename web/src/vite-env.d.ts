/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_FLEET_GRAPHQL_URL?: string;
  readonly VITE_AGENT_BASE_URL?: string;
  readonly VITE_POLL_INTERVAL_MS?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
