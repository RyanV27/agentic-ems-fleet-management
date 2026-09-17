package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryansamvarghese/agentic-fleet-management/services/fleet/internal/domain"
)

func TestUnit_InsertGetUpdateRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.InsertZone(ctx, domain.Zone{ID: "zone-1", Name: "Downtown"}))

	callID := "call-1"
	unit := domain.Unit{
		ID:              "unit-01",
		Callsign:        "Medic 1",
		Status:          domain.UnitStatusAvailable,
		Capability:      domain.CapabilityALS,
		ZoneID:          "zone-1",
		CurrentCallID:   nil,
		StatusChangedAt: normalizeTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	require.NoError(t, s.InsertUnit(ctx, unit))

	got, err := s.GetUnit(ctx, unit.ID)
	require.NoError(t, err)
	assert.Equal(t, unit, got)

	unit.Status = domain.UnitStatusEnRoute
	unit.CurrentCallID = &callID
	unit.StatusChangedAt = unit.StatusChangedAt.Add(time.Minute)
	require.NoError(t, s.UpdateUnit(ctx, unit))

	got, err = s.GetUnit(ctx, unit.ID)
	require.NoError(t, err)
	assert.Equal(t, unit, got)
}

func TestUnit_ListOrderedByID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.InsertZone(ctx, domain.Zone{ID: "zone-1", Name: "Downtown"}))

	for _, id := range []string{"unit-03", "unit-01", "unit-02"} {
		require.NoError(t, s.InsertUnit(ctx, domain.Unit{
			ID: id, Callsign: id, Status: domain.UnitStatusAvailable,
			Capability: domain.CapabilityALS, ZoneID: "zone-1", StatusChangedAt: time.Now().UTC(),
		}))
	}

	units, err := s.ListUnits(ctx)
	require.NoError(t, err)
	require.Len(t, units, 3)
	assert.Equal(t, []string{"unit-01", "unit-02", "unit-03"}, []string{units[0].ID, units[1].ID, units[2].ID})
}

func TestUnit_GetMissingReturnsNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetUnit(context.Background(), "does-not-exist")
	require.Error(t, err)
}

// TestUnit_InvalidEnumFailsToLoadClearly asserts S1 c5: an invalid enum
// string in the DB fails to load with a clear error rather than silently
// producing a zero-value or garbage domain.UnitStatus.
func TestUnit_InvalidEnumFailsToLoadClearly(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.InsertZone(ctx, domain.Zone{ID: "zone-1", Name: "Downtown"}))

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO units (id, callsign, status, capability, zone_id, current_call_id, status_changed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"unit-bad", "Bad Unit", "NOT_A_REAL_STATUS", string(domain.CapabilityALS), "zone-1", nil,
		timeToString(time.Now().UTC()))
	require.NoError(t, err)

	_, err = s.GetUnit(ctx, "unit-bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid unit status")
}
