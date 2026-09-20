package actions_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/actions"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// TestExecuteAction_AssignsCallAndDequeues covers the ASSIGN_CALL happy path:
// unit and call mutate, and the call leaves the operator queue (DEC-021).
func TestExecuteAction_AssignsCallAndDequeues(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, q := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))
	q.Push(dispatch.QueueItem{CallID: call.ID, Severity: call.Severity, EnqueuedAt: *call.EnqueuedAt}, now)

	proposed, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	executed, err := mgr.ExecuteAction(ctx, proposed.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusExecuted, executed.Status)
	require.NotNil(t, executed.DecidedBy)
	assert.Equal(t, testOperatorID, *executed.DecidedBy)

	afterUnit, err := st.GetUnit(ctx, unit.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.UnitStatusEnRoute, afterUnit.Status)
	require.NotNil(t, afterUnit.CurrentCallID)
	assert.Equal(t, call.ID, *afterUnit.CurrentCallID)

	afterCall, err := st.GetCall(ctx, call.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusAssigned, afterCall.Status)
	require.NotNil(t, afterCall.AssignedUnitID)
	assert.Equal(t, unit.ID, *afterCall.AssignedUnitID)

	_, ok := q.Peek(now)
	assert.False(t, ok, "executing ASSIGN_CALL must dequeue the call")
}

// TestExecuteAction_SystemAndAgentSameCodePath is ROADMAP c3: DecidedBy is
// always OperatorID regardless of proposedBy.
func TestExecuteAction_SystemAndAgentSameCodePath(t *testing.T) {
	for _, proposedBy := range []domain.ProposedBy{domain.ProposedBySystem, domain.ProposedByAgent} {
		t.Run(string(proposedBy), func(t *testing.T) {
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			clock := &fakeClock{now: now}
			mgr, st, _ := newTestManager(t, clock)
			ctx := context.Background()

			mustInsertZone(t, st, "zone-1")
			unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
			call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))

			proposed, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, actions.ProposeInput{ProposedBy: proposedBy})
			require.NoError(t, err)

			executed, err := mgr.ExecuteAction(ctx, proposed.ID)
			require.NoError(t, err)
			require.NotNil(t, executed.DecidedBy)
			assert.Equal(t, testOperatorID, *executed.DecidedBy, "DecidedBy must be OperatorID regardless of proposedBy")
		})
	}
}

// TestExecuteAction_PreconditionFailureMarksFailedAndLeavesFleetUnchanged is
// ROADMAP c4: fleet state drifted since propose (unit was taken by another
// assignment), so execute must revalidate, mark FAILED, and change nothing.
func TestExecuteAction_PreconditionFailureMarksFailedAndLeavesFleetUnchanged(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call1 := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))

	proposed, err := mgr.ProposeAssignCall(ctx, call1.ID, unit.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	// Drift: the unit goes OUT_OF_SERVICE out-of-band (not through a manager
	// mutation, so no event-driven invalidation fires) between propose and
	// execute — execute must catch this by revalidating live state, not by
	// trusting the snapshot the proposal was created against.
	unit.Status = domain.UnitStatusOutOfService
	require.NoError(t, st.UpdateUnit(ctx, unit))

	beforeUnit, err := st.GetUnit(ctx, unit.ID)
	require.NoError(t, err)
	beforeCall1, err := st.GetCall(ctx, call1.ID)
	require.NoError(t, err)

	executed, err := mgr.ExecuteAction(ctx, proposed.ID)
	require.Error(t, err)
	var precondErr *actions.PreconditionError
	assert.ErrorAs(t, err, &precondErr)
	assert.Equal(t, domain.ActionStatusFailed, executed.Status)

	afterUnit, err := st.GetUnit(ctx, unit.ID)
	require.NoError(t, err)
	afterCall1, err := st.GetCall(ctx, call1.ID)
	require.NoError(t, err)
	assert.Equal(t, beforeUnit, afterUnit, "fleet state must be unchanged after a failed precondition")
	assert.Equal(t, beforeCall1, afterCall1, "fleet state must be unchanged after a failed precondition")

	stored, err := st.GetPendingAction(ctx, proposed.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusFailed, stored.Status)
}

// TestExecuteAction_AlreadyDecidedIsIllegal is ROADMAP c8: executing (or
// rejecting) an action that is not PROPOSED must fail, with no state change.
func TestExecuteAction_AlreadyDecidedIsIllegal(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))

	proposed, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	_, err = mgr.ExecuteAction(ctx, proposed.ID)
	require.NoError(t, err)

	// Executing an already-EXECUTED action must fail.
	_, err = mgr.ExecuteAction(ctx, proposed.ID)
	require.Error(t, err)
	var transErr *actions.TransitionError
	assert.ErrorAs(t, err, &transErr)

	// Rejecting an already-EXECUTED action must fail too.
	_, err = mgr.RejectAction(ctx, proposed.ID, domain.RejectionReasonOther, nil)
	require.Error(t, err)
	assert.ErrorAs(t, err, &transErr)

	stored, err := st.GetPendingAction(ctx, proposed.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionStatusExecuted, stored.Status, "status must remain EXECUTED, not overwritten")
}

// TestExecuteAction_ExecutingExpiredIsIllegal covers the EXPIRED branch of c8.
func TestExecuteAction_ExecutingExpiredIsIllegal(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))

	proposed, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	clock.now = now.Add(time.Duration(testTTLSeconds+1) * time.Second)
	expired, err := mgr.SweepExpired(ctx, clock.now)
	require.NoError(t, err)
	require.Len(t, expired, 1)

	_, err = mgr.ExecuteAction(ctx, proposed.ID)
	require.Error(t, err)
	var transErr *actions.TransitionError
	assert.ErrorAs(t, err, &transErr)

	_, err = mgr.RejectAction(ctx, proposed.ID, domain.RejectionReasonOther, nil)
	require.Error(t, err)
	assert.ErrorAs(t, err, &transErr)
}

// TestExecuteAction_IdempotencyKeyCollisionReturnsSameRow is part of c5:
// proposing the same subject twice (no explicit key) must not create a
// second row to execute independently.
func TestExecuteAction_IdempotencyKeyCollisionReturnsSameRow(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))

	in := actions.ProposeInput{ProposedBy: domain.ProposedBySystem}
	first, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, in)
	require.NoError(t, err)
	second, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, in)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)

	_, err = mgr.ExecuteAction(ctx, first.ID)
	require.NoError(t, err)

	all, err := st.ListPendingActions(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 1, "idempotency-key collision must not leave a second independently-executable row")
}

// TestExecuteAction_ConcurrentExecutesOnlyOneSucceeds is c5's concurrency
// requirement: N goroutines racing to execute the same PROPOSED action must
// result in exactly one EXECUTED transition and the rest failing on the
// PROPOSED-only guard, never a double mutation.
func TestExecuteAction_ConcurrentExecutesOnlyOneSucceeds(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))

	proposed, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	const n = 20
	var wg sync.WaitGroup
	var successCount int32
	var mu sync.Mutex
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := mgr.ExecuteAction(ctx, proposed.ID)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), successCount, "exactly one concurrent execute must succeed")

	afterUnit, err := st.GetUnit(ctx, unit.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.UnitStatusEnRoute, afterUnit.Status)
}

// TestExecuteAction_RerouteUnit covers the REROUTE_UNIT effect (DEC-037):
// the unit's previous call reopens (preserving EnqueuedAt) and re-enters the
// queue; the target call becomes ASSIGNED and leaves the queue.
func TestExecuteAction_RerouteUnit(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, q := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	originalEnqueue := now.Add(-5 * time.Minute)
	previousCall := openCall("call-prev", "zone-1", 2, now)
	previousCall.EnqueuedAt = &originalEnqueue
	previousCall.Status = domain.CallStatusAssigned
	mustInsertCall(t, st, previousCall)

	targetCall := mustInsertCall(t, st, openCall("call-target", "zone-1", 1, now))
	q.Push(dispatch.QueueItem{CallID: targetCall.ID, Severity: targetCall.Severity, EnqueuedAt: *targetCall.EnqueuedAt}, now)

	unit := availableUnit("unit-1", "zone-1", domain.CapabilityALS, now)
	unit.Status = domain.UnitStatusEnRoute
	prevID := previousCall.ID
	unit.CurrentCallID = &prevID
	mustInsertUnit(t, st, unit)

	// AssignedUnitID back-reference on the previous call.
	unitID := unit.ID
	previousCall.AssignedUnitID = &unitID
	require.NoError(t, st.UpdateCall(ctx, previousCall))

	proposed, err := mgr.ProposeRerouteUnit(ctx, unit.ID, targetCall.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	_, err = mgr.ExecuteAction(ctx, proposed.ID)
	require.NoError(t, err)

	afterPrev, err := st.GetCall(ctx, previousCall.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusPendingApproval, afterPrev.Status)
	assert.Nil(t, afterPrev.AssignedUnitID)
	require.NotNil(t, afterPrev.EnqueuedAt)
	assert.True(t, afterPrev.EnqueuedAt.Equal(originalEnqueue), "EnqueuedAt must be preserved, not reset (DEC-021)")

	afterTarget, err := st.GetCall(ctx, targetCall.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusAssigned, afterTarget.Status)

	afterUnit, err := st.GetUnit(ctx, unit.ID)
	require.NoError(t, err)
	assert.Equal(t, targetCall.ID, *afterUnit.CurrentCallID)

	// Target call dequeued, previous call re-enqueued.
	item, ok := q.Peek(now)
	require.True(t, ok)
	assert.Equal(t, previousCall.ID, item.CallID)
}

// TestExecuteAction_AssignBackupUnit covers ASSIGN_BACKUP_UNIT (DEC-037):
// the previously assigned unit is freed to AVAILABLE, the backup takes over.
func TestExecuteAction_AssignBackupUnit(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 2, now))
	call.Status = domain.CallStatusAssigned

	original := availableUnit("unit-original", "zone-1", domain.CapabilityBLS, now)
	original.Status = domain.UnitStatusOutOfService
	callID := call.ID
	original.CurrentCallID = &callID
	mustInsertUnit(t, st, original)

	originalID := original.ID
	call.AssignedUnitID = &originalID
	require.NoError(t, st.UpdateCall(ctx, call))

	backup := mustInsertUnit(t, st, availableUnit("unit-backup", "zone-1", domain.CapabilityBLS, now))

	proposed, err := mgr.ProposeAssignBackupUnit(ctx, call.ID, backup.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	_, err = mgr.ExecuteAction(ctx, proposed.ID)
	require.NoError(t, err)

	afterOriginal, err := st.GetUnit(ctx, original.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.UnitStatusAvailable, afterOriginal.Status)
	assert.Nil(t, afterOriginal.CurrentCallID)

	afterBackup, err := st.GetUnit(ctx, backup.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.UnitStatusEnRoute, afterBackup.Status)

	afterCall, err := st.GetCall(ctx, call.ID)
	require.NoError(t, err)
	assert.Equal(t, backup.ID, *afterCall.AssignedUnitID)
}

// TestExecuteAction_ResolveEvent covers RESOLVE_EVENT.
func TestExecuteAction_ResolveEvent(t *testing.T) {
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

	proposed, err := mgr.ProposeResolveEvent(ctx, event.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	_, err = mgr.ExecuteAction(ctx, proposed.ID)
	require.NoError(t, err)

	after, err := st.GetDispatchEvent(ctx, event.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.DispatchEventStatusResolved, after.Status)
	assert.NotNil(t, after.ResolvedAt)
}

// TestExecuteAction_ParentActionIDRoundTrips is c11: parentActionId round-
// trips through the store but has no execution effect in Phase 1.
func TestExecuteAction_ParentActionIDRoundTrips(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))

	parentID := "parent-action-id"
	proposed, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, actions.ProposeInput{
		ProposedBy:     domain.ProposedBySystem,
		ParentActionID: &parentID,
	})
	require.NoError(t, err)
	require.NotNil(t, proposed.ParentActionID)
	assert.Equal(t, parentID, *proposed.ParentActionID)

	stored, err := st.GetPendingAction(ctx, proposed.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.ParentActionID)
	assert.Equal(t, parentID, *stored.ParentActionID)
}
