package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate_CreatesAllTables(t *testing.T) {
	s := newTestStore(t)

	want := []string{
		"units", "calls", "extractions", "zones", "zone_travel_times", "hospitals",
		"dispatch_events", "pending_actions", "routing_decisions", "agent_runs", "tool_call_logs",
	}
	for _, table := range want {
		var name string
		err := s.db.QueryRowContext(context.Background(),
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		require.NoError(t, err, "table %s should exist", table)
		assert.Equal(t, table, name)
	}
}

func TestMigrate_IsIdempotent(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Migrate(context.Background()))
	require.NoError(t, s.Migrate(context.Background()))
}

func TestMigrate_FromEmptyFile(t *testing.T) {
	// newTestStore migrates once already; asserting no error here is the
	// "forward from an empty file" half of S1 c3.
	s, err := Open("file:empty-migrate-test?mode=memory&cache=shared")
	require.NoError(t, err)
	defer func() { _ = s.Close() }()
	require.NoError(t, s.Migrate(context.Background()))
}
