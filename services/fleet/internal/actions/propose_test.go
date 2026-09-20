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

// TestPropose_OnlyInsertsProposedRow is ROADMAP S6 c1: proposing must not
// mutate any fleet state (call/unit/event rows) or any other pending_actions
// row — it inserts exactly one new PROPOSED row.
func TestPropose_OnlyInsertsProposedRow(t *testing.T) {
	for _, proposedBy := range []domain.ProposedBy{domain.ProposedBySystem, domain.ProposedByAgent, domain.ProposedByOperator} {
		t.Run(string(proposedBy), func(t *testing.T) {
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			clock := &fakeClock{now: now}
			mgr, st, _ := newTestManager(t, clock)
			ctx := context.Background()

			mustInsertZone(t, st, "zone-1")
			unit := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
			call := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))

			beforeCall, err := st.GetCall(ctx, call.ID)
			require.NoError(t, err)
			beforeUnit, err := st.GetUnit(ctx, unit.ID)
			require.NoError(t, err)
			beforeActions, err := st.ListPendingActions(ctx)
			require.NoError(t, err)
			require.Empty(t, beforeActions)

			proposed, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, actions.ProposeInput{
				ProposedBy: proposedBy,
				Rationale:  "nearest available ALS",
			})
			require.NoError(t, err)
			assert.Equal(t, domain.ActionStatusProposed, proposed.Status)
			assert.Equal(t, proposedBy, proposed.ProposedBy)

			afterCall, err := st.GetCall(ctx, call.ID)
			require.NoError(t, err)
			afterUnit, err := st.GetUnit(ctx, unit.ID)
			require.NoError(t, err)
			assert.Equal(t, beforeCall, afterCall, "propose must not mutate the call")
			assert.Equal(t, beforeUnit, afterUnit, "propose must not mutate the unit")

			afterActions, err := st.ListPendingActions(ctx)
			require.NoError(t, err)
			require.Len(t, afterActions, 1, "propose must insert exactly one row")
			assert.Equal(t, proposed.ID, afterActions[0].ID)
		})
	}
}

func TestPropose_AllActionTypes(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, st, _ := newTestManager(t, clock)
	ctx := context.Background()

	mustInsertZone(t, st, "zone-1")
	unit1 := mustInsertUnit(t, st, availableUnit("unit-1", "zone-1", domain.CapabilityALS, now))
	unit2 := mustInsertUnit(t, st, availableUnit("unit-2", "zone-1", domain.CapabilityALS, now))
	call1 := mustInsertCall(t, st, openCall("call-1", "zone-1", 1, now))
	call2 := mustInsertCall(t, st, openCall("call-2", "zone-1", 1, now))
	event := mustInsertDispatchEvent(t, st, domain.DispatchEvent{
		ID:        "event-1",
		Type:      domain.DispatchEventTypeBreakdown,
		Status:    domain.DispatchEventStatusOpen,
		CreatedAt: now,
	})

	// Give unit2 a current call so reroute has something to move it off of.
	callID2 := call2.ID
	unit2.CurrentCallID = &callID2
	unit2.Status = domain.UnitStatusEnRoute
	require.NoError(t, st.UpdateUnit(ctx, unit2))
	call2.Status = domain.CallStatusAssigned
	unitID2 := unit2.ID
	call2.AssignedUnitID = &unitID2
	require.NoError(t, st.UpdateCall(ctx, call2))

	in := actions.ProposeInput{ProposedBy: domain.ProposedBySystem, Rationale: "test"}

	a1, err := mgr.ProposeAssignCall(ctx, call1.ID, unit1.ID, in)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionTypeAssignCall, a1.Type)

	a2, err := mgr.ProposeRerouteUnit(ctx, unit2.ID, call1.ID, in)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionTypeRerouteUnit, a2.Type)

	a3, err := mgr.ProposeAssignBackupUnit(ctx, call2.ID, unit1.ID, in)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionTypeAssignBackupUnit, a3.Type)

	a4, err := mgr.ProposeResolveEvent(ctx, event.ID, in)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionTypeResolveEvent, a4.Type)

	all, err := st.ListPendingActions(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 4)
}

func TestPropose_NonexistentSubjectFailsFast(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	mgr, _, _ := newTestManager(t, clock)
	ctx := context.Background()

	_, err := mgr.ProposeAssignCall(ctx, "no-such-call", "no-such-unit", actions.ProposeInput{
		ProposedBy: domain.ProposedBySystem,
	})
	assert.Error(t, err)
}

func TestPropose_DefaultIdempotencyKeyDerivation(t *testing.T) {
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

	// Re-proposing the identical (type, subject) pair with no explicit key
	// must collide on the derived default key, per InsertPendingAction's
	// idempotency semantics (return existing row, not an error).
	second, err := mgr.ProposeAssignCall(ctx, call.ID, unit.ID, in)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "same subject with no explicit key must collide on the derived default")

	all, err := st.ListPendingActions(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 1)
}
