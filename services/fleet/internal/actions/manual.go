package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// Manual mutations are the one exception to "only executeAction changes
// fleet state" (DEC-016): an operator-initiated assignCall, rerouteUnit, or
// resolveDispatchEvent mutates directly, with no PROPOSED phase, because the
// operator's click *is* the approval. Each still writes an EXECUTED
// pending_actions row with proposedBy = OPERATOR for audit (ROADMAP S6 c9),
// sharing the exact same mutation core and precondition checks executeAction
// uses, so a manual assignment can never diverge from what an approved
// proposal would have done.

// ManualAssignCall assigns unitID to callID immediately (no proposal). It
// runs the same precondition check and mutation as an ASSIGN_CALL
// executeAction, in one transaction, and records an EXECUTED audit row.
func (m *Manager) ManualAssignCall(ctx context.Context, callID, unitID string) (domain.PendingAction, error) {
	payload, err := marshalPayload(AssignCallPayload{CallID: callID, UnitID: unitID})
	if err != nil {
		return domain.PendingAction{}, err
	}
	return m.manualMutate(ctx, domain.ActionTypeAssignCall, payload, "manual operator assignment",
		func(ctx context.Context, now time.Time) (executeOutcome, error, error) {
			return m.mutateAssignCall(ctx, callID, unitID, "", now)
		})
}

// ManualRerouteUnit reroutes unitID onto callID immediately (no proposal).
func (m *Manager) ManualRerouteUnit(ctx context.Context, unitID, callID string) (domain.PendingAction, error) {
	payload, err := marshalPayload(RerouteUnitPayload{UnitID: unitID, CallID: callID})
	if err != nil {
		return domain.PendingAction{}, err
	}
	return m.manualMutate(ctx, domain.ActionTypeRerouteUnit, payload, "manual operator reroute",
		func(ctx context.Context, now time.Time) (executeOutcome, error, error) {
			return m.mutateRerouteUnit(ctx, unitID, callID, "", now)
		})
}

// ManualResolveDispatchEvent resolves a dispatch event immediately (no
// proposal).
func (m *Manager) ManualResolveDispatchEvent(ctx context.Context, dispatchEventID string) (domain.PendingAction, error) {
	payload, err := marshalPayload(ResolveEventPayload{DispatchEventID: dispatchEventID})
	if err != nil {
		return domain.PendingAction{}, err
	}
	return m.manualMutate(ctx, domain.ActionTypeResolveEvent, payload, "manual operator event resolution",
		func(ctx context.Context, now time.Time) (executeOutcome, error, error) {
			return m.mutateResolveEvent(ctx, dispatchEventID, "", now)
		})
}

// manualMutate runs mutate in a transaction and, on success, records an
// EXECUTED audit row with proposedBy = OPERATOR. A precondition failure
// aborts the transaction with no audit row and no fleet mutation — unlike
// executeAction, a manual mutation that fails its precondition was never
// proposed, so there is nothing to mark FAILED.
func (m *Manager) manualMutate(ctx context.Context, actionType domain.ActionType, payload []byte, rationale string,
	mutate func(ctx context.Context, now time.Time) (executeOutcome, error, error)) (domain.PendingAction, error) {

	now := m.Clock.Now()
	var result domain.PendingAction
	var outcome executeOutcome

	err := m.Store.WithTx(ctx, func(ctx context.Context) error {
		out, precondErr, err := mutate(ctx, now)
		if err != nil {
			return err
		}
		if precondErr != nil {
			return precondErr
		}
		outcome = out

		decidedBy := m.OperatorID
		a := domain.PendingAction{
			ID:             uuid.NewString(),
			Type:           actionType,
			Payload:        payload,
			Status:         domain.ActionStatusExecuted,
			IdempotencyKey: fmt.Sprintf("MANUAL:%s", uuid.NewString()),
			ProposedBy:     domain.ProposedByOperator,
			Rationale:      rationale,
			CreatedAt:      now,
			DecidedAt:      &now,
			DecidedBy:      &decidedBy,
		}
		inserted, err := m.Store.InsertPendingAction(ctx, a)
		if err != nil {
			return fmt.Errorf("actions: manual %s: record audit row: %w", actionType, err)
		}
		result = inserted
		return nil
	})
	if err != nil {
		return domain.PendingAction{}, err
	}

	m.applyOutcome(outcome, now)
	return result, nil
}
