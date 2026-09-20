package actions_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/actions"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store/sqlite"
)

// testDBCounter guarantees a unique in-memory DSN per test, mirroring the
// pattern in internal/store/sqlite/sqlite_test.go.
var testDBCounter int64

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	n := atomic.AddInt64(&testDBCounter, 1)
	dsn := fmt.Sprintf("file:actions-test-%d?mode=memory&cache=shared", n)
	s, err := sqlite.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	require.NoError(t, s.Migrate(context.Background()))
	return s
}

func testQueueConfig() dispatch.PriorityConfig {
	return dispatch.PriorityConfig{
		SeverityWeights: map[int]float64{1: 1000, 2: 100, 3: 10},
		AgingRate:       1.0,
	}
}

// fakeClock is a settable Clock for deterministic time control in tests.
type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

const testOperatorID = "operator-1"
const testTTLSeconds = 600

func newTestManager(t *testing.T, clock actions.Clock) (*actions.Manager, *sqlite.Store, *dispatch.Queue) {
	t.Helper()
	st := newTestStore(t)
	q := dispatch.NewQueue(testQueueConfig())
	mgr := actions.NewManager(st, q, clock, testOperatorID, testTTLSeconds)
	return mgr, st, q
}

func mustInsertZone(t *testing.T, st *sqlite.Store, id string) {
	t.Helper()
	require.NoError(t, st.InsertZone(context.Background(), domain.Zone{ID: id, Name: id}))
}

func mustInsertUnit(t *testing.T, st *sqlite.Store, u domain.Unit) domain.Unit {
	t.Helper()
	require.NoError(t, st.InsertUnit(context.Background(), u))
	return u
}

func mustInsertCall(t *testing.T, st *sqlite.Store, c domain.Call) domain.Call {
	t.Helper()
	require.NoError(t, st.InsertCall(context.Background(), c))
	return c
}

func mustInsertDispatchEvent(t *testing.T, st *sqlite.Store, e domain.DispatchEvent) domain.DispatchEvent {
	t.Helper()
	require.NoError(t, st.InsertDispatchEvent(context.Background(), e))
	return e
}

func availableUnit(id, zoneID string, cap domain.Capability, now time.Time) domain.Unit {
	return domain.Unit{
		ID:              id,
		Callsign:        id,
		Status:          domain.UnitStatusAvailable,
		Capability:      cap,
		ZoneID:          zoneID,
		StatusChangedAt: now,
	}
}

func openCall(id, zoneID string, severity int, now time.Time) domain.Call {
	enqueuedAt := now
	return domain.Call{
		ID:         id,
		Transcript: "test transcript",
		Severity:   severity,
		ZoneID:     zoneID,
		Status:     domain.CallStatusPendingApproval,
		CreatedAt:  now,
		EnqueuedAt: &enqueuedAt,
	}
}

func dispatchQueueItem(c domain.Call) dispatch.QueueItem {
	return dispatch.QueueItem{CallID: c.ID, Severity: c.Severity, EnqueuedAt: *c.EnqueuedAt}
}
