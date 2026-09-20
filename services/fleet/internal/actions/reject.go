package actions

import (
	"context"
	"fmt"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// RejectAction records an operator's structured rejection of a PROPOSED
// action (FR-21, ROADMAP S6 c10). It changes no fleet state and does not
// touch the operator queue — the call the action targeted is still open and
// still needs a decision, so it must remain visible and queued.
func (m *Manager) RejectAction(ctx context.Context, actionID string, reason domain.RejectionReason, note *string) (domain.PendingAction, error) {
	if !reason.Valid() {
		return domain.PendingAction{}, fmt.Errorf("actions: reject action %s: invalid rejection reason %q", actionID, reason)
	}

	now := m.Clock.Now()
	var result domain.PendingAction
	err := m.Store.WithTx(ctx, func(ctx context.Context) error {
		a, err := m.Store.GetPendingAction(ctx, actionID)
		if err != nil {
			return fmt.Errorf("actions: reject action %s: %w", actionID, err)
		}
		if a.Status != domain.ActionStatusProposed {
			return &TransitionError{ActionID: a.ID, From: a.Status, To: domain.ActionStatusRejected}
		}
		a.Status = domain.ActionStatusRejected
		a.RejectionReason = &reason
		a.RejectionNote = note
		decidedAt := now
		decidedBy := m.OperatorID
		a.DecidedAt = &decidedAt
		a.DecidedBy = &decidedBy
		if err := m.Store.UpdatePendingAction(ctx, a); err != nil {
			return fmt.Errorf("actions: reject action %s: %w", a.ID, err)
		}
		result = a
		return nil
	})
	return result, err
}
