package resolver

import (
	"context"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// RejectAction records a structured rejection (FR-21, ROADMAP S6 c10). It
// mutates no fleet state and does not dequeue the call.
func (r *mutationResolver) RejectAction(ctx context.Context, actionID string, reason generated.RejectionReason, note *string) (*generated.PendingAction, error) {
	a, err := r.Manager.RejectAction(ctx, actionID, domain.RejectionReason(reason), note)
	if err != nil {
		return nil, err
	}
	return mapPendingAction(a), nil
}
