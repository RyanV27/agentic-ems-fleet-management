package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/config"
)

// TestBuildApp_SeedsAndServesGraphQL proves the production wiring in
// buildApp (store, clock, rules, dispatch queue, approval-gate manager,
// intake pipeline, resolver) produces a working GraphQL server: a fresh DB
// is seeded on first boot, and a real /query request round-trips through
// the actual resolver stack. This is the automated check CLAUDE.md requires
// for cmd/server/main.go (ROADMAP S7 c9 needs a runnable service).
func TestBuildApp_SeedsAndServesGraphQL(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	a, err := buildApp(ctx, cfg)
	require.NoError(t, err)
	defer func() { _ = a.store.Close() }()

	zones, err := a.store.ListZones(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, zones, "buildApp must seed a fresh DB")

	body := postQuery(t, a, `{"query":"{ zones { id name } }"}`)
	require.Contains(t, body, `"zones"`)
	require.NotContains(t, body, `"errors"`)
}

// TestBuildApp_SeedIsIdempotentAcrossRestarts proves store.Seed's
// non-idempotency (fixed-ID rows) cannot crash a second boot against the
// same persistent DB — the seedIfEmpty guard in main.go must hold.
func TestBuildApp_SeedIsIdempotentAcrossRestarts(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	first, err := buildApp(ctx, cfg)
	require.NoError(t, err)
	zonesAfterFirstBoot, err := first.store.ListZones(ctx)
	require.NoError(t, err)
	require.NoError(t, first.store.Close())

	second, err := buildApp(ctx, cfg)
	require.NoError(t, err)
	defer func() { _ = second.store.Close() }()

	zonesAfterSecondBoot, err := second.store.ListZones(ctx)
	require.NoError(t, err)
	require.Len(t, zonesAfterSecondBoot, len(zonesAfterFirstBoot), "restart must not re-seed or duplicate rows")
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.FleetDBPath = filepath.Join(t.TempDir(), "fleet.db")
	cfg.AgentEnabled = false // this test never submits a transcript; keep it offline-safe regardless
	return cfg
}

func postQuery(t *testing.T, a *app, body string) string {
	t.Helper()
	req := httptest.NewRequest("POST", "/query", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code, rec.Body.String())

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed))
	out, err := json.Marshal(parsed)
	require.NoError(t, err)
	return string(out)
}
