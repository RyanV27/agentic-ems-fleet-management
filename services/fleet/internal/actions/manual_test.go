package actions_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// TestManualAssignCall_MutatesDirectlyAndRecordsExecutedAudit is ROADMAP c9:
// a manual mutation needs no proposal, no agent, and no API key — the
// operator's call mutates the fleet immediately and leaves an EXECUTED,
// proposedBy=OPERATOR audit row.
func TestManualAssignCall_MutatesDirectlyAndRecordsExecutedAudit(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, q := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))
	q.Push(dispatchQueueItem(call), now)

	before, err := st.ListPendingActions(ctx)
	require.NoError(t, err)
	require.Empty(t, before, "no proposal should exist before a manual mutation")

	audit, err := mgr.ManualAssignCall(ctx, call.ID, unit.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusExecuted, audit.Status)
	assert.Equal(t, domain.ProposedByOperator, audit.ProposedBy)
	require.NotNil(t, audit.DecidedBy)
	assert.Equal(t, testOperatorID, *audit.DecidedBy)

	afterUnit, err := st.GetUnit(ctx, unit.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.UnitStatusEnRoute, afterUnit.Status)

	afterCall, err := st.GetCall(ctx, call.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusAssigned, afterCall.Status)

	_, ok := q.Peek(now)
	assert.False(t, ok, "manual assignment must dequeue the call")

	all, err := st.ListPendingActions(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, domain.ActionStatusExecuted, all[0].Status)
}

// TestManualRerouteUnit_MutatesDirectly covers the manual REROUTE_UNIT path.
func TestManualRerouteUnit_MutatesDirectly(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	previousCall := openCall("call-prev", "zone-1", 2, now)
	previousCall.Status = domain.CallStatusAssigned
	mustInsertCall(t, st, previousCall)
	targetCall := mustInsertCall(t, st, openCall("call-target", "zone-1", 1, now))

	unit := availableUnit("unit-1", "zone-1", domain.CapabilityALS, now)
	unit.Status = domain.UnitStatusEnRoute
	prevID := previousCall.ID
	unit.CurrentCallID = &prevID
	mustInsertUnit(t, st, unit)

	audit, err := mgr.ManualRerouteUnit(ctx, unit.ID, targetCall.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusExecuted, audit.Status)
	assert.Equal(t, domain.ProposedByOperator, audit.ProposedBy)

	afterUnit, err := st.GetUnit(ctx, unit.ID)
	require.NoError(t, err)
	assert.Equal(t, targetCall.ID, *afterUnit.CurrentCallID)

	afterPrev, err := st.GetCall(ctx, previousCall.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusPendingApproval, afterPrev.Status)
}

// TestManualResolveDispatchEvent_MutatesDirectly covers the manual
// RESOLVE_EVENT path.
func TestManualResolveDispatchEvent_MutatesDirectly(t *testing.T) {
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

	audit, err := mgr.ManualResolveDispatchEvent(ctx, event.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusExecuted, audit.Status)
	assert.Equal(t, domain.ProposedByOperator, audit.ProposedBy)

	after, err := st.GetDispatchEvent(ctx, event.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.DispatchEventStatusResolved, after.Status)
}

// TestManualAssignCall_PreconditionFailureRecordsNoAuditRow: a manual
// mutation whose precondition fails was never proposed, so — unlike
// executeAction — there is nothing to mark FAILED; no audit row, no mutation.
func TestManualAssignCall_PreconditionFailureRecordsNoAuditRow(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := availableUnit("unit-1", "zone-1", domain.CapabilityALS, now)
	unit.Status = domain.UnitStatusOutOfService
	mustInsertUnit(t, st, unit)
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))

	_, err := mgr.ManualAssignCall(ctx, call.ID, unit.ID)
	assert.Error(t, err)

	all, err := st.ListPendingActions(ctx)
	require.NoError(t, err)
	assert.Empty(t, all, "a failed manual mutation must not leave an audit row")

	afterCall, err := st.GetCall(ctx, call.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusPendingApproval, afterCall.Status, "fleet state must be unchanged")
}
