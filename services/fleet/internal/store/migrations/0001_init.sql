-- Initial schema: every entity in ARCHITECTURE.md §4 (ROADMAP.md S1 c1).
-- Booleans and enums are stored as TEXT/INTEGER; domain.<Enum>.Valid() is the
-- validation boundary on load (S1 c5). Timestamps are RFC3339Nano UTC text.

CREATE TABLE IF NOT EXISTS zones (
    id   TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS zone_travel_times (
    from_zone_id   TEXT NOT NULL REFERENCES zones(id),
    to_zone_id     TEXT NOT NULL REFERENCES zones(id),
    travel_seconds INTEGER NOT NULL,
    PRIMARY KEY (from_zone_id, to_zone_id)
);

CREATE TABLE IF NOT EXISTS hospitals (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    zone_id      TEXT NOT NULL REFERENCES zones(id),
    capabilities TEXT NOT NULL, -- JSON array of strings
    accepting    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS units (
    id                TEXT PRIMARY KEY,
    callsign          TEXT NOT NULL,
    status            TEXT NOT NULL,
    capability        TEXT NOT NULL,
    zone_id           TEXT NOT NULL REFERENCES zones(id),
    current_call_id   TEXT,
    status_changed_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS calls (
    id                      TEXT PRIMARY KEY,
    transcript              TEXT NOT NULL,
    severity                INTEGER NOT NULL,
    zone_id                 TEXT NOT NULL REFERENCES zones(id),
    status                  TEXT NOT NULL,
    assigned_unit_id        TEXT,
    destination_hospital_id TEXT,
    created_at              TEXT NOT NULL,
    enqueued_at             TEXT
);

CREATE TABLE IF NOT EXISTS extractions (
    id                TEXT PRIMARY KEY,
    call_id           TEXT NOT NULL UNIQUE REFERENCES calls(id),
    incident_type     TEXT NOT NULL,
    severity          INTEGER NOT NULL,
    zone_id           TEXT NOT NULL,
    keywords          TEXT NOT NULL, -- JSON array of strings
    needs_transport   INTEGER NOT NULL,
    confidence        REAL NOT NULL,
    failure_reason    TEXT,
    model             TEXT,
    latency_ms        INTEGER,
    prompt_tokens     INTEGER,
    completion_tokens INTEGER,
    raw_response      TEXT,
    created_at        TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS dispatch_events (
    id             TEXT PRIMARY KEY,
    type           TEXT NOT NULL,
    call_id        TEXT,
    unit_id        TEXT,
    severity_delta INTEGER,
    status         TEXT NOT NULL,
    note           TEXT,
    created_at     TEXT NOT NULL,
    resolved_at    TEXT
);

CREATE TABLE IF NOT EXISTS pending_actions (
    id               TEXT PRIMARY KEY,
    type             TEXT NOT NULL,
    payload          TEXT NOT NULL, -- JSON
    status           TEXT NOT NULL,
    idempotency_key  TEXT NOT NULL UNIQUE,
    proposed_by      TEXT NOT NULL,
    rationale        TEXT,
    expired_reason   TEXT,
    agent_run_id     TEXT,
    rejection_reason TEXT,
    rejection_note   TEXT,
    parent_action_id TEXT,
    created_at       TEXT NOT NULL,
    decided_at       TEXT,
    decided_by       TEXT
);

CREATE TABLE IF NOT EXISTS routing_decisions (
    id              TEXT PRIMARY KEY,
    call_id         TEXT NOT NULL REFERENCES calls(id),
    path            TEXT NOT NULL,
    matched_rule_id TEXT NOT NULL,
    input_snapshot  TEXT NOT NULL, -- JSON
    decided_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS agent_runs (
    id                TEXT PRIMARY KEY,
    call_id           TEXT NOT NULL REFERENCES calls(id),
    trigger           TEXT NOT NULL,
    attempt           INTEGER NOT NULL,
    status            TEXT NOT NULL,
    started_at        TEXT NOT NULL,
    ended_at          TEXT,
    latency_ms        INTEGER,
    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd          REAL NOT NULL DEFAULT 0,
    stop_reason       TEXT
);

CREATE TABLE IF NOT EXISTS tool_call_logs (
    id           TEXT PRIMARY KEY,
    agent_run_id TEXT NOT NULL REFERENCES agent_runs(id),
    seq          INTEGER NOT NULL,
    tool_name    TEXT NOT NULL,
    input        TEXT NOT NULL, -- JSON
    output       TEXT,          -- JSON
    latency_ms   INTEGER NOT NULL,
    error        TEXT
);
