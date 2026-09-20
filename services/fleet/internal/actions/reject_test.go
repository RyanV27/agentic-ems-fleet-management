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

// TestRejectAction_RecordsStructuredReasonAndLeavesFleetUnchanged is
// ROADMAP c10 (FR-21): rejecting a proposal records a structured reason and
// an optional note, mutates no fleet state, and does not dequeue the call.
func TestRejectAction_RecordsStructuredReasonAndLeavesFleetUnchanged(t *testing.T) {
	for _, reason := range []domain.RejectionReason{
		domain.RejectionReasonWrongUnit,
		domain.RejectionReasonInsufficientInfo,
		domain.RejectionReasonUnsafeTiming,
		domain.RejectionReasonOther,
	} {
		t.Run(string(reason), func(t *testing.T) {
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

			beforeCall, err := st.GetCall(ctx, call.ID)
			require.NoError(t, err)
			beforeUnit, err := st.GetUnit(ctx, unit.ID)
			require.NoError(t, err)

			note := "not the right call"
			rejected, err := mgr.RejectAction(ctx, proposed.ID, reason, &note)
			require.NoError(t, err)
			assert.Equal(t, domain.ActionStatusRejected, rejected.Status)
			require.NotNil(t, rejected.RejectionReason)
			assert.Equal(t, reason, *rejected.RejectionReason)
			require.NotNil(t, rejected.RejectionNote)
			assert.Equal(t, note, *rejected.RejectionNote)
			require.NotNil(t, rejected.DecidedBy)
			assert.Equal(t, testOperatorID, *rejected.DecidedBy)

			afterCall, err := st.GetCall(ctx, call.ID)
			require.NoError(t, err)
			afterUnit, err := st.GetUnit(ctx, unit.ID)
			require.NoError(t, err)
			assert.Equal(t, beforeCall, afterCall, "reject must not mutate the call")
			assert.Equal(t, beforeUnit, afterUnit, "reject must not mutate the unit")

			item, ok := q.Peek(now)
			require.True(t, ok, "rejecting must not dequeue — the call still needs a decision")
			assert.Equal(t, call.ID, item.CallID)
		})
	}
}

// TestRejectAction_InvalidReasonIsRejected guards the enum boundary.
func TestRejectAction_InvalidReasonIsRejected(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))

	proposed, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, actions.ProposeInput{ProposedBy: domain.ProposedBySystem})
	require.NoError(t, err)

	_, err = mgr.RejectAction(ctx, proposed.ID, domain.RejectionReason("NOT_A_REAL_REASON"), nil)
	assert.Error(t, err)
}
