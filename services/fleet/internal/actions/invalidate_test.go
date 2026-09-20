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

// TestInvalidate_UnitReassignedExpiresOtherProposalsForThatUnit is DEC-019
// trigger 1, exercised via the execute-time side effect (ROADMAP c6).
func TestInvalidate_UnitReassignedExpiresOtherProposalsForThatUnit(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call1 := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))
	call2 := mustInsertCall(t, st, openCall("call-2", "zone-1", 1, now))

	// Two competing proposals for the same unit, targeting different calls.
	winner, err := mgr.ProposeAssignCall(ctx, call1.ID, unit.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)
	loser, err := mgr.ProposeAssignCall(ctx, call2.ID, unit.ID, actions.ProposeInput{ProposedBy: domain.ProposedByAgent})
	require.NoError(t, err)

	_, err = mgr.ExecuteAction(ctx, winner.ID)
	require.NoError(t, err)

	stored, err := st.GetPendingAction(ctx, loser.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusExpired, stored.Status)
	require.NotNil(t, stored.ExpiredReason)
	assert.Equal(t, domain.ExpiredReasonUnitReassigned, *stored.ExpiredReason)
	assert.NotEqual(t, domain.ExpiredReasonTTL, *stored.ExpiredReason, "an event-driven trigger fired, not the TTL backstop")
}

// TestInvalidate_SupersededFiresAtExecuteNotPropose is the DEC-037
// resolution of DEC-019 trigger 3's literal wording: Propose* must not
// invalidate anything (ROADMAP c1); supersession is a side effect of a
// successful execute.
func TestInvalidate_SupersededFiresAtExecuteNotPropose(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit1 := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	unit2 := mustInsertUnit(t, st, availableUnit("unit-2", "zone-1", domain.CapabilityALS, now))
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))

	a1, err := mgr.ProposeAssignCall(ctx, call.ID, unit1.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)
	a2, err := mgr.ProposeAssignCall(ctx, call.ID, unit2.ID, actions.ProposeInput{ProposedBy: domain.ProposedByAgent})
	require.NoError(t, err)

	// Proposing a2 must not have touched a1 yet.
	stillProposed, err := st.GetPendingAction(ctx, a1.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusProposed, stillProposed.Status, "propose must never invalidate another proposal")

	_, err = mgr.ExecuteAction(ctx, a2.ID)
	require.NoError(t, err)

	afterExecute, err := st.GetPendingAction(ctx, a1.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusExpired, afterExecute.Status, "execute must supersede the other proposal for the same call")
	require.NotNil(t, afterExecute.ExpiredReason)
	assert.Equal(t, domain.ExpiredReasonSuperseded, *afterExecute.ExpiredReason)
}

// TestInvalidate_UnitOutOfServiceExpiresProposalsTargetingIt is DEC-019
// trigger 1's OUT_OF_SERVICE case, via the internal-only helper (DEC-037).
func TestInvalidate_UnitOutOfServiceExpiresProposalsTargetingIt(t *testing.T) {
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

	stored, err := st.GetPendingAction(ctx, proposed.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusExpired, stored.Status)
	require.NotNil(t, stored.ExpiredReason)
	assert.Equal(t, domain.ExpiredReasonUnitOutOfService, *stored.ExpiredReason)
}

// TestInvalidate_CallClosedExpiresProposalsAndDequeues is DEC-019 trigger 2
// and ROADMAP c8b: closing/cancelling a call is a legitimate dequeue trigger,
// unlike ordinary proposal expiry.
func TestInvalidate_CallClosedExpiresProposalsAndDequeues(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, q := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))
	q.Push(dispatchQueueItem(call), now)

	proposed, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	require.NoError(t, mgr.InvalidateCallClosed(ctx, call.ID, now))

	stored, err := st.GetPendingAction(ctx, proposed.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusExpired, stored.Status)
	require.NotNil(t, stored.ExpiredReason)
	assert.Equal(t, domain.ExpiredReasonCallClosed, *stored.ExpiredReason)

	_, ok := q.Peek(now)
	assert.False(t, ok, "closing a call must dequeue it")
}

// TestInvalidate_EventSupersededOnExecute covers RESOLVE_EVENT's supersede
// trigger.
func TestInvalidate_EventSupersededOnExecute(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	event := mustInsertDispatchEvent(t, st, domain.DispatchEvent{
		ID:        "event-1",
		Type:      domain.DispatchEventTypeBreakdown,
		Status:    domain.DispatchEventStatusOpen,
		CreatedAt: now,
	})

	a1, err := mgr.ProposeResolveEvent(ctx, event.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem, IdempotencyKey: "manual-1"})
	require.NoError(t, err)
	a2, err := mgr.ProposeResolveEvent(ctx, event.ID, actions.ProposeInput{ProposedBy: domain.ProposedByAgent, IdempotencyKey: "manual-2"})
	require.NoError(t, err)

	_, err = mgr.ExecuteAction(ctx, a2.ID)
	require.NoError(t, err)

	stored, err := st.GetPendingAction(ctx, a1.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusExpired, stored.Status)
	require.NotNil(t, stored.ExpiredReason)
	assert.Equal(t, domain.ExpiredReasonSuperseded, *stored.ExpiredReason)
}
