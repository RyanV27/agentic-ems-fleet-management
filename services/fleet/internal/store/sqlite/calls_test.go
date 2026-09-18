package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

func seedOneZone(t *testing.T, s *Store) {
	t.Helper()
	require.NoError(t, s.InsertZone(context.Background(), domain.Zone{ID: "zone-1", Name: "Downtown"}))
}

func TestCall_InsertGetRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedOneZone(t, s)

	call := domain.Call{
		ID:         "call-1",
		Transcript: "chest pain, 45yo male",
		Severity:   1,
		ZoneID:     "zone-1",
		Status:     domain.CallStatusEscalated,
		CreatedAt:  normalizeTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		EnqueuedAt: nil,
	}
	require.NoError(t, s.InsertCall(ctx, call))

	got, err := s.GetCall(ctx, call.ID)
	require.NoError(t, err)
	assert.Equal(t, call, got)
}

// TestCall_EnqueuedAtSetExactlyOnce is S1 criterion 8: enqueuedAt is the
// sole aging input and must never be overwritten once set.
func TestCall_EnqueuedAtSetExactlyOnce(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedOneZone(t, s)

	call := domain.Call{
		ID: "call-1", Transcript: "t", Severity: 2, ZoneID: "zone-1",
		Status: domain.CallStatusEscalated, CreatedAt: normalizeTime(time.Now().UTC()),
	}
	require.NoError(t, s.InsertCall(ctx, call))

	first := normalizeTime(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	set, err := s.SetCallEnqueuedAt(ctx, call.ID, first)
	require.NoError(t, err)
	assert.True(t, set, "first call should set enqueued_at")

	got, err := s.GetCall(ctx, call.ID)
	require.NoError(t, err)
	require.NotNil(t, got.EnqueuedAt)
	assert.Equal(t, first, *got.EnqueuedAt)

	second := normalizeTime(time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC))
	set, err = s.SetCallEnqueuedAt(ctx, call.ID, second)
	require.NoError(t, err)
	assert.False(t, set, "second call must not overwrite enqueued_at")

	got, err = s.GetCall(ctx, call.ID)
	require.NoError(t, err)
	require.NotNil(t, got.EnqueuedAt)
	assert.Equal(t, first, *got.EnqueuedAt, "enqueued_at must remain the first value written")
}

func TestCall_UpdateDoesNotTouchEnqueuedAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedOneZone(t, s)

	call := domain.Call{
		ID: "call-1", Transcript: "t", Severity: 2, ZoneID: "zone-1",
		Status: domain.CallStatusEscalated, CreatedAt: normalizeTime(time.Now().UTC()),
	}
	require.NoError(t, s.InsertCall(ctx, call))

	now := normalizeTime(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	_, err := s.SetCallEnqueuedAt(ctx, call.ID, now)
	require.NoError(t, err)

	call.Status = domain.CallStatusAssigned
	require.NoError(t, s.UpdateCall(ctx, call))

	got, err := s.GetCall(ctx, call.ID)
	require.NoError(t, err)
	require.NotNil(t, got.EnqueuedAt)
	assert.Equal(t, now, *got.EnqueuedAt)
	assert.Equal(t, domain.CallStatusAssigned, got.Status)
}
