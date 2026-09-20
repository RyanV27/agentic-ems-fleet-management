package resolver

import (
	"context"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
)

// ExecuteAction is the only mutation that can change fleet state for a
// system/agent-originated proposal (DEC-016). All business logic — precondition
// revalidation, the fleet mutation, and the state transition — lives in
// internal/actions.Manager.ExecuteAction; this resolver only maps.
func (r *mutationResolver) ExecuteAction(ctx context.Context, actionID string) (*generated.PendingAction, error) {
	a, err := r.Manager.ExecuteAction(ctx, actionID)
	if err != nil {
		return nil, err
	}
	return mapPendingAction(a), nil
}
