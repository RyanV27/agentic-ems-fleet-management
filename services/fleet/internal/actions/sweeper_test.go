package actions_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/actions"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// TestSweepExpired_ExpiresOnlyPastTTL is ROADMAP c7: the TTL backstop only
// catches PROPOSED actions strictly at/past TTLSeconds old, and does so with
// reason TTL — never overlapping with an event-driven rule's reason.
func TestSweepExpired_ExpiresOnlyPastTTL(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit1 := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	unit2 := mustInsertUnit(t, st, availableUnit("unit-2", "zone-1", domain.CapabilityALS, now))
	call1 := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))
	call2 := mustInsertCall(t, st, openCall("call-2", "zone-1", 1, now))

	old, err := mgr.ProposeAssignCall(ctx, call1.ID, unit1.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	// Advance the clock past TTL, then propose a second, fresh action.
	later := now.Add(time.Duration(testTTLSeconds+1) * time.Second)
	clock.now = later
	fresh, err := mgr.ProposeAssignCall(ctx, call2.ID, unit2.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	expired, err := mgr.SweepExpired(ctx, later)
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, old.ID, expired[0].ID)

	storedOld, err := st.GetPendingAction(ctx, old.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusExpired, storedOld.Status)
	require.NotNil(t, storedOld.ExpiredReason)
	assert.Equal(t, domain.ExpiredReasonTTL, *storedOld.ExpiredReason)

	storedFresh, err := st.GetPendingAction(ctx, fresh.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusProposed, storedFresh.Status, "a fresh proposal must not be swept")
}

// TestSweepExpired_DoesNotDequeue is ROADMAP c8b: TTL expiry, unlike
// execute/manual-assign/close, must never remove the call from the queue —
// the call is still open and still needs a decision.
func TestSweepExpired_DoesNotDequeue(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, q := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))
	q.Push(dispatchQueueItem(call), now)

	_, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	later := now.Add(time.Duration(testTTLSeconds+1) * time.Second)
	_, err = mgr.SweepExpired(ctx, later)
	require.NoError(t, err)

	item, ok := q.Peek(later)
	require.True(t, ok, "TTL expiry must not dequeue the call")
	assert.Equal(t, call.ID, item.CallID)
}

// TestSweepExpired_TTLBackstopDoesNotOverlapEventDrivenRule verifies that
// once an event-driven rule has already invalidated a proposal, the TTL
// sweep does not double-expire it or overwrite its reason.
func TestSweepExpired_TTLBackstopDoesNotOverlapEventDrivenRule(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))

	proposed, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	require.NoError(t, mgr.InvalidateUnitOutOfService(ctx, unit.ID, now))

	later := now.Add(time.Duration(testTTLSeconds+1) * time.Second)
	expired, err := mgr.SweepExpired(ctx, later)
	require.NoError(t, err)
	assert.Empty(t, expired, "an already-EXPIRED action is not PROPOSED, so the sweeper must skip it")

	stored, err := st.GetPendingAction(ctx, proposed.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.ExpiredReason)
	assert.Equal(t, domain.ExpiredReasonUnitOutOfService, *stored.ExpiredReason, "the original event-driven reason must survive, not be overwritten by TTL")
}
