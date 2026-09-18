package sqlite

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

func newTestAction(id, idempotencyKey string) domain.PendingAction {
	return domain.PendingAction{
		ID:             id,
		Type:           domain.ActionTypeAssignCall,
		Payload:        json.RawMessage(`{"unitId":"unit-01"}`),
		Status:         domain.ActionStatusProposed,
		IdempotencyKey: idempotencyKey,
		ProposedBy:     domain.ProposedBySystem,
		Rationale:      "nearest available ALS",
		CreatedAt:      normalizeTime(time.Now().UTC()),
	}
}

// TestPendingAction_IdempotencyKeyUnique is S1 criterion 2: a duplicate
// idempotency_key returns the existing row's id rather than an error.
func TestPendingAction_IdempotencyKeyUnique(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	first := newTestAction("action-1", "idem-key-1")
	got1, err := s.InsertPendingAction(ctx, first)
	require.NoError(t, err)
	assert.Equal(t, "action-1", got1.ID)

	second := newTestAction("action-2", "idem-key-1")
	got2, err := s.InsertPendingAction(ctx, second)
	require.NoError(t, err)
	assert.Equal(t, "action-1", got2.ID, "duplicate idempotency key must return the existing row's id")

	all, err := s.ListPendingActions(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestPendingAction_IdempotencyKeyUnique_Concurrent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	const n = 20
	var wg sync.WaitGroup
	ids := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a := newTestAction("action-concurrent-"+string(rune('a'+i)), "shared-idem-key")
			got, err := s.InsertPendingAction(ctx, a)
			ids[i] = got.ID
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	all, err := s.ListPendingActions(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 1, "exactly one row should exist for the shared idempotency key")

	first := ids[0]
	for _, id := range ids {
		assert.Equal(t, first, id, "every concurrent insert must resolve to the same action id")
	}
}

func TestPendingAction_GetUpdateRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := newTestAction("action-1", "idem-1")
	_, err := s.InsertPendingAction(ctx, a)
	require.NoError(t, err)

	got, err := s.GetPendingAction(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, a.Status, got.Status)
	assert.Equal(t, a.Rationale, got.Rationale)
	assert.Nil(t, got.ExpiredReason)
	assert.Nil(t, got.RejectionReason)

	decidedAt := normalizeTime(time.Now().UTC())
	decidedBy := "operator-1"
	got.Status = domain.ActionStatusExecuted
	got.DecidedAt = &decidedAt
	got.DecidedBy = &decidedBy
	require.NoError(t, s.UpdatePendingAction(ctx, got))

	reloaded, err := s.GetPendingAction(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusExecuted, reloaded.Status)
	require.NotNil(t, reloaded.DecidedAt)
	assert.Equal(t, decidedAt, *reloaded.DecidedAt)
	require.NotNil(t, reloaded.DecidedBy)
	assert.Equal(t, decidedBy, *reloaded.DecidedBy)
}

// TestPendingAction_InvalidNullableEnumFailsToLoadClearly asserts S1 c5 on a
// nullable enum column (expired_reason), not just the non-nullable
// units.status case already covered in units_test.go.
func TestPendingAction_InvalidNullableEnumFailsToLoadClearly(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := newTestAction("action-1", "idem-1")
	_, err := s.InsertPendingAction(ctx, a)
	require.NoError(t, err)

	_, err = s.db.ExecContext(ctx,
		`UPDATE pending_actions SET expired_reason = ? WHERE id = ?`,
		"NOT_A_REAL_EXPIRED_REASON", a.ID)
	require.NoError(t, err)

	_, err = s.GetPendingAction(ctx, a.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid expired reason")
}

func TestPendingAction_ExpiredReasonRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := newTestAction("action-1", "idem-1")
	_, err := s.InsertPendingAction(ctx, a)
	require.NoError(t, err)

	got, err := s.GetPendingAction(ctx, a.ID)
	require.NoError(t, err)

	reason := domain.ExpiredReasonTTL
	got.Status = domain.ActionStatusExpired
	got.ExpiredReason = &reason
	require.NoError(t, s.UpdatePendingAction(ctx, got))

	reloaded, err := s.GetPendingAction(ctx, a.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.ExpiredReason)
	assert.Equal(t, domain.ExpiredReasonTTL, *reloaded.ExpiredReason)
}
